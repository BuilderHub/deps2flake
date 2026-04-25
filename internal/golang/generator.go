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

type nopherRunner interface {
	generate(ctx context.Context, sourceDir, outputDir string, force bool) error
}

type generator struct {
	runner nopherRunner
}

// New creates a Go generator.
func New(nopherBin string) scaffold.Generator {
	if nopherBin == "" {
		nopherBin = defaultNopherBin
	}
	return &generator{runner: commandRunner{Binary: nopherBin}}
}

func newWithRunner(runner nopherRunner) scaffold.Generator {
	return &generator{runner: runner}
}

// Detect reports whether dir looks like a Go module.
func (g *generator) Detect(_ context.Context, dir string) (bool, error) {
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
func (g *generator) Generate(ctx context.Context, req scaffold.Request) (scaffold.Result, error) {
	meta, err := parseModule(filepath.Join(req.Dir, "go.mod"))
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

	if err := g.runner.generate(ctx, req.Dir, req.OutputDir, req.Force); err != nil {
		return scaffold.Result{}, err
	}

	if err := flake.WriteGo(flakePath, flake.GoData{
		PackageName:      meta.PackageName,
		ModulePath:       meta.ModulePath,
		GoVersion:        meta.GoVersion,
		SubPackages:      normalizeGoSubPackages(req.Go.SubPackages),
		IncludeContainer: req.IncludeContainer,
	}, req.Force); err != nil {
		return scaffold.Result{}, err
	}

	return scaffold.Result{
		FlakePath:    flakePath,
		LockfilePath: lockfilePath,
	}, nil
}

func normalizeGoSubPackages(subPackages []string) []string {
	if len(subPackages) == 0 {
		return []string{"./"}
	}

	normalized := make([]string, 0, len(subPackages))
	for _, subPackage := range subPackages {
		subPackage = strings.TrimSpace(subPackage)
		switch subPackage {
		case "", ".", "./":
			normalized = append(normalized, "./")
		default:
			if strings.HasPrefix(subPackage, "./") {
				normalized = append(normalized, subPackage)
				continue
			}
			normalized = append(normalized, "./"+subPackage)
		}
	}
	return normalized
}

type module struct {
	ModulePath  string
	GoVersion   string
	PackageName string
}

func parseModule(path string) (module, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return module{}, fmt.Errorf("reading go.mod: %w", err)
	}

	parsed, err := modfile.Parse(path, data, nil)
	if err != nil {
		return module{}, fmt.Errorf("parsing go.mod: %w", err)
	}
	if parsed.Module == nil || parsed.Module.Mod.Path == "" {
		return module{}, fmt.Errorf("go.mod does not declare a module path")
	}

	modulePath := parsed.Module.Mod.Path
	goVersion := ""
	if parsed.Go != nil {
		goVersion = parsed.Go.Version
	}

	return module{
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

type commandRunner struct {
	Binary string
}

func (r commandRunner) generate(ctx context.Context, sourceDir, outputDir string, force bool) error {
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

func (r commandRunner) generateInDir(ctx context.Context, binary, dir string) error {
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
