// Package golang implements Go project support.
package golang

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/BuilderHub/deps2flake/internal/flake"
	"github.com/BuilderHub/deps2flake/internal/scaffold"
	nophergen "github.com/anthr76/nopher/pkg/generator"
	"golang.org/x/mod/modfile"
)

type lockfileRunner interface {
	generate(ctx context.Context, sourceDir, outputDir string, force bool) error
}

type generator struct {
	runner lockfileRunner
}

// New creates a Go generator.
func New() scaffold.Generator {
	return &generator{runner: pkgRunner{opts: nophergen.Options{}}}
}

// NewWithNopherOptions returns a Go generator that uses the given nopher options
// (for example, a test-only Fetch stub).
func NewWithNopherOptions(opts nophergen.Options) scaffold.Generator {
	return &generator{runner: pkgRunner{opts: opts}}
}

func newWithRunner(runner lockfileRunner) scaffold.Generator {
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

	goOpts := req.Go
	goOpts.SubPackages = normalizeGoSubPackages(req.Go.SubPackages)

	if err := flake.WriteGo(flakePath, flake.GoData{
		PackageName:      meta.PackageName,
		ModulePath:       meta.ModulePath,
		GoVersion:        meta.GoVersion,
		IncludeContainer: req.IncludeContainer,
		GoOptions:        goOpts,
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

type pkgRunner struct {
	opts nophergen.Options
}

func (r pkgRunner) generate(_ context.Context, sourceDir, outputDir string, force bool) error {
	workDir := sourceDir
	if sourceDir != outputDir {
		tmpDir, err := os.MkdirTemp("", "deps2flake-nopher-*")
		if err != nil {
			return fmt.Errorf("creating nopher workspace: %w", err)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		for _, name := range []string{"go.mod", "go.sum"} {
			if err := copyFile(filepath.Join(sourceDir, name), filepath.Join(tmpDir, name)); err != nil {
				return err
			}
		}
		workDir = tmpDir
	}

	lf, err := nophergen.Generate(workDir, r.opts)
	if err != nil {
		return fmt.Errorf("generating nopher lockfile: %w", err)
	}

	tmpYAML, err := os.CreateTemp("", "deps2flake-nopher-lock-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp lockfile: %w", err)
	}
	tmpPath := tmpYAML.Name()
	if err := tmpYAML.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing temp lockfile: %w", err)
	}
	defer func() { _ = os.Remove(tmpPath) }()

	if err := lf.SaveYAML(tmpPath); err != nil {
		return fmt.Errorf("writing temp lockfile: %w", err)
	}

	dest := filepath.Join(outputDir, "nopher.lock.yaml")
	if sourceDir != outputDir {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
	}
	return copyGeneratedLockfile(tmpPath, dest, force)
}

func copyFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filepath.Base(source), err)
	}
	if err := os.WriteFile(destination, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", filepath.Base(destination), err)
	}
	return nil
}

func copyGeneratedLockfile(source, destination string, force bool) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("reading generated lockfile: %w", err)
	}
	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !force {
		flag |= os.O_EXCL
	}
	if err := writeFile(destination, data, flag, 0644); err != nil {
		return fmt.Errorf("creating lockfile: %w", err)
	}
	return nil
}

func writeFile(path string, data []byte, flag int, perm os.FileMode) error {
	file, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return nil
}
