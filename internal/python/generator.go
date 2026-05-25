// Package python implements Python (uv/pyproject) project support.
package python

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BuilderHub/deps2flake/internal/flake"
	"github.com/BuilderHub/deps2flake/internal/naming"
	"github.com/BuilderHub/deps2flake/internal/scaffold"
	"github.com/pelletier/go-toml/v2"
)

type lockfileRunner interface {
	ensure(ctx context.Context, sourceDir, outputDir string, force bool) error
}

type generator struct {
	runner lockfileRunner
}

// New creates a Python generator.
func New() scaffold.Generator {
	return &generator{runner: uvLockRunner{}}
}

// NewWithLockRunner returns a Python generator that uses the given lockfile runner.
func NewWithLockRunner(runner lockfileRunner) scaffold.Generator {
	return &generator{runner: runner}
}

// TestLockRunner skips uv lock and copies project files when output dir differs.
type TestLockRunner struct{}

func (TestLockRunner) ensure(_ context.Context, sourceDir, outputDir string, force bool) error {
	if sourceDir == outputDir {
		return nil
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	for _, name := range []string{"pyproject.toml", "uv.lock"} {
		if err := copyProjectFile(filepath.Join(sourceDir, name), filepath.Join(outputDir, name), force); err != nil {
			return err
		}
	}
	return nil
}

// NewForTest returns a Python generator suitable for CLI and integration tests.
func NewForTest() scaffold.Generator {
	return newWithRunner(TestLockRunner{})
}

func newWithRunner(runner lockfileRunner) scaffold.Generator {
	return &generator{runner: runner}
}

// Detect reports whether dir looks like a uv/pyproject Python project.
func (g *generator) Detect(_ context.Context, dir string) (bool, error) {
	info, err := os.Stat(filepath.Join(dir, "pyproject.toml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

// Generate creates uv.lock (if needed) and flake.nix for a Python project.
func (g *generator) Generate(ctx context.Context, req scaffold.Request) (scaffold.Result, error) {
	meta, err := parseProject(filepath.Join(req.Dir, "pyproject.toml"))
	if err != nil {
		return scaffold.Result{}, err
	}

	flakePath := filepath.Join(req.OutputDir, "flake.nix")
	lockfilePath := filepath.Join(req.OutputDir, "uv.lock")

	if err := flake.EnsureWritable(flakePath, req.Force); err != nil {
		return scaffold.Result{}, err
	}
	if req.OutputDir != req.Dir {
		if err := flake.EnsureWritable(lockfilePath, req.Force); err != nil {
			return scaffold.Result{}, err
		}
	}

	if err := g.runner.ensure(ctx, req.Dir, req.OutputDir, req.Force); err != nil {
		return scaffold.Result{}, err
	}

	pyOpts := req.Python
	if pyOpts.SourcePreference == "" {
		pyOpts.SourcePreference = "wheel"
	}

	mainProgram := meta.PackageName
	if len(pyOpts.Scripts) > 0 {
		mainProgram = pyOpts.Scripts[0]
	}

	if err := flake.WritePython(flakePath, flake.PythonData{
		PackageName:      meta.PackageName,
		ProjectName:      meta.ProjectName,
		ProjectVersion:   meta.ProjectVersion,
		PythonVersion:    meta.PythonVersion,
		MainProgram:      mainProgram,
		IncludeContainer: req.IncludeContainer,
		PythonOptions:    pyOpts,
	}, req.Force); err != nil {
		return scaffold.Result{}, err
	}

	return scaffold.Result{
		FlakePath:    flakePath,
		LockfilePath: lockfilePath,
	}, nil
}

type projectMeta struct {
	ProjectName    string
	ProjectVersion string
	PythonVersion  string
	PackageName    string
}

type pyprojectFile struct {
	Project struct {
		Name           string `toml:"name"`
		Version        string `toml:"version"`
		RequiresPython string `toml:"requires-python"`
	} `toml:"project"`
}

func parseProject(path string) (projectMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return projectMeta{}, fmt.Errorf("reading pyproject.toml: %w", err)
	}

	var parsed pyprojectFile
	if err := toml.Unmarshal(data, &parsed); err != nil {
		return projectMeta{}, fmt.Errorf("parsing pyproject.toml: %w", err)
	}
	if parsed.Project.Name == "" {
		return projectMeta{}, fmt.Errorf("pyproject.toml does not declare [project].name")
	}

	version := parsed.Project.Version
	if version == "" {
		version = "0.1.0"
	}

	return projectMeta{
		ProjectName:    parsed.Project.Name,
		ProjectVersion: version,
		PythonVersion:    parsed.Project.RequiresPython,
		PackageName:      naming.PackageName(parsed.Project.Name),
	}, nil
}

type uvLockRunner struct{}

func (uvLockRunner) ensure(ctx context.Context, sourceDir, outputDir string, force bool) error {
	sourceLock := filepath.Join(sourceDir, "uv.lock")
	if _, err := os.Stat(sourceLock); errors.Is(err, os.ErrNotExist) {
		cmd := exec.CommandContext(ctx, "uv", "lock")
		cmd.Dir = sourceDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("running uv lock: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("reading uv.lock: %w", err)
	}

	if sourceDir == outputDir {
		return nil
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	for _, name := range []string{"pyproject.toml", "uv.lock"} {
		if err := copyProjectFile(filepath.Join(sourceDir, name), filepath.Join(outputDir, name), force); err != nil {
			return err
		}
	}
	return nil
}

func copyProjectFile(source, destination string, force bool) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filepath.Base(source), err)
	}
	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !force {
		flag |= os.O_EXCL
	}
	file, err := os.OpenFile(destination, flag, 0644)
	if err != nil {
		return fmt.Errorf("writing %s: %w", filepath.Base(destination), err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("writing %s: %w", filepath.Base(destination), err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", filepath.Base(destination), err)
	}
	return nil
}
