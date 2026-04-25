// Package golang implements Go project support.
package golang

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/BuilderHub/deps2flake/internal/flake"
	"github.com/BuilderHub/deps2flake/internal/scaffold"
	"golang.org/x/mod/modfile"
)

const defaultNopherBin = "nopher"

// NopherRunner produces the nopher.lock.yaml file for a Go project.
type NopherRunner interface {
	Generate(ctx context.Context, sourceDir, outputDir string, force bool) error
}

// Generator generates flakes for Go projects.
type Generator struct {
	runner NopherRunner
}

// New creates a Go generator.
func New(nopherBin string) *Generator {
	if nopherBin == "" {
		nopherBin = defaultNopherBin
	}
	return &Generator{runner: CommandRunner{Binary: nopherBin}}
}

// NewWithRunner creates a Go generator with an injected nopher runner.
func NewWithRunner(runner NopherRunner) *Generator {
	return &Generator{runner: runner}
}

// Detect reports whether dir looks like a Go module.
func (g *Generator) Detect(_ context.Context, dir string) (bool, error) {
	info, err := os.Stat(filepath.Join(dir, "go.mod"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

// Generate creates nopher.lock.yaml and flake.nix for a Go project.
func (g *Generator) Generate(ctx context.Context, req scaffold.Request) (scaffold.Result, error) {
	meta, err := ParseModule(filepath.Join(req.Dir, "go.mod"))
	if err != nil {
		return scaffold.Result{}, err
	}

	flakePath := filepath.Join(req.OutputDir, "flake.nix")
	lockfilePath := filepath.Join(req.OutputDir, "nopher.lock.yaml")

	if err := flake.EnsureWritable(flakePath, req.Force); err != nil {
		return scaffold.Result{}, err
	}
	if err := flake.EnsureWritable(lockfilePath, req.Force); err != nil {
		return scaffold.Result{}, err
	}

	if err := g.runner.Generate(ctx, req.Dir, req.OutputDir, req.Force); err != nil {
		return scaffold.Result{}, err
	}

	if err := flake.WriteGo(flakePath, flake.GoData{
		PackageName:      meta.PackageName,
		ModulePath:       meta.ModulePath,
		GoVersion:        meta.GoVersion,
		SubPackages:      []string{"."},
		IncludeContainer: req.IncludeContainer,
	}, req.Force); err != nil {
		return scaffold.Result{}, err
	}

	return scaffold.Result{
		FlakePath:    flakePath,
		LockfilePath: lockfilePath,
	}, nil
}

// Module describes the Go module metadata needed for a generated flake.
type Module struct {
	ModulePath  string
	GoVersion   string
	PackageName string
}

// ParseModule reads module metadata from go.mod.
func ParseModule(path string) (Module, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Module{}, fmt.Errorf("reading go.mod: %w", err)
	}

	parsed, err := modfile.Parse(path, data, nil)
	if err != nil {
		return Module{}, fmt.Errorf("parsing go.mod: %w", err)
	}
	if parsed.Module == nil || parsed.Module.Mod.Path == "" {
		return Module{}, fmt.Errorf("go.mod does not declare a module path")
	}

	modulePath := parsed.Module.Mod.Path
	goVersion := ""
	if parsed.Go != nil {
		goVersion = parsed.Go.Version
	}

	return Module{
		ModulePath:  modulePath,
		GoVersion:   goVersion,
		PackageName: packageName(modulePath),
	}, nil
}

func packageName(modulePath string) string {
	base := filepath.Base(modulePath)
	base = strings.TrimSpace(strings.ToLower(base))

	var b strings.Builder
	lastDash := false
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	name := strings.Trim(b.String(), "-")
	if name == "" {
		return "app"
	}
	if name[0] >= '0' && name[0] <= '9' {
		return "app-" + name
	}
	return name
}

// CommandRunner shells out to the nopher CLI.
type CommandRunner struct {
	Binary string
}

// Generate runs `nopher generate` and writes nopher.lock.yaml to outputDir.
func (r CommandRunner) Generate(ctx context.Context, sourceDir, outputDir string, force bool) error {
	binary := r.Binary
	if binary == "" {
		binary = defaultNopherBin
	}

	if sourceDir == outputDir {
		return r.generateInDir(ctx, binary, sourceDir)
	}

	tmpDir, err := os.MkdirTemp("", "deps2flake-nopher-*")
	if err != nil {
		return fmt.Errorf("creating nopher workspace: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	for _, name := range []string{"go.mod", "go.sum"} {
		if err := copyFile(filepath.Join(sourceDir, name), filepath.Join(tmpDir, name)); err != nil {
			return err
		}
	}

	if err := r.generateInDir(ctx, binary, tmpDir); err != nil {
		return err
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	lockfilePath := filepath.Join(outputDir, "nopher.lock.yaml")
	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !force {
		flag |= os.O_EXCL
	}
	out, err := os.OpenFile(lockfilePath, flag, 0644)
	if err != nil {
		return fmt.Errorf("creating lockfile: %w", err)
	}

	in, err := os.Open(filepath.Join(tmpDir, "nopher.lock.yaml"))
	if err != nil {
		_ = out.Close()
		return fmt.Errorf("opening generated lockfile: %w", err)
	}
	defer func() {
		_ = in.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copying lockfile: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("closing lockfile: %w", err)
	}
	return nil
}

func (r CommandRunner) generateInDir(ctx context.Context, binary, dir string) error {
	cmd := exec.CommandContext(ctx, binary, "generate", dir)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("nopher executable %q not found; install github.com/anthr76/nopher or pass --nopher-bin: %w", binary, err)
	}

	message := strings.TrimSpace(string(output))
	if message == "" {
		return fmt.Errorf("running %s generate: %w", binary, err)
	}
	return fmt.Errorf("running %s generate: %w\n%s", binary, err, message)
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("opening %s: %w", filepath.Base(source), err)
	}
	defer func() {
		_ = in.Close()
	}()

	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Base(destination), err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copying %s: %w", filepath.Base(source), err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", filepath.Base(destination), err)
	}
	return nil
}
