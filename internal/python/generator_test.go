package python

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BuilderHub/deps2flake/internal/scaffold"
)

func TestDetectPyproject(t *testing.T) {
	dir := t.TempDir()
	generator := newWithRunner(noopRunner{})

	ok, err := generator.Detect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("empty directory detected as Python project")
	}

	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(`
[project]
name = "demo"
version = "1.0.0"
`), 0644); err != nil {
		t.Fatal(err)
	}

	ok, err = generator.Detect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("pyproject.toml was not detected")
	}
}

func TestParseProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyproject.toml")
	if err := os.WriteFile(path, []byte(`
[project]
name = "My_App"
version = "2.3.4"
requires-python = ">=3.12"
`), 0644); err != nil {
		t.Fatal(err)
	}

	meta, err := parseProject(path)
	if err != nil {
		t.Fatal(err)
	}
	if meta.ProjectName != "My_App" {
		t.Fatalf("project name = %q", meta.ProjectName)
	}
	if meta.ProjectVersion != "2.3.4" {
		t.Fatalf("version = %q", meta.ProjectVersion)
	}
	if meta.PythonVersion != ">=3.12" {
		t.Fatalf("requires-python = %q", meta.PythonVersion)
	}
	if meta.PackageName != "my-app" {
		t.Fatalf("package name = %q", meta.PackageName)
	}
}

func TestParseProjectErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := parseProject(filepath.Join(t.TempDir(), "pyproject.toml"))
		if err == nil {
			t.Fatal("expected missing file error")
		}
		if !strings.Contains(err.Error(), "reading pyproject.toml") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "pyproject.toml")
		if err := os.WriteFile(path, []byte("[project]\nversion = \"1.0.0\"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		_, err := parseProject(path)
		if err == nil {
			t.Fatal("expected missing name error")
		}
	})
}

func TestGenerateWritesFlakeAndLockfile(t *testing.T) {
	dir := t.TempDir()
	writePyproject(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "uv.lock"), []byte("version = 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	generator := newWithRunner(noopRunner{})
	result, err := generator.Generate(context.Background(), req(dir, dir, false))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(result.FlakePath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(result.LockfilePath); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(result.FlakePath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, "nopher.url") {
		t.Fatalf("python flake must not reference nopher:\n%s", got)
	}
	if !strings.Contains(got, `url = "github:pyproject-nix/uv2nix"`) {
		t.Fatalf("python flake missing uv2nix:\n%s", got)
	}
}

func TestGenerateCopiesLockfileToOutputDir(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "dist")
	writePyproject(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "uv.lock"), []byte("version = 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	generator := newWithRunner(copyRunner{})
	_, err := generator.Generate(context.Background(), req(dir, out, false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "uv.lock")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "pyproject.toml")); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateDoesNotOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	writePyproject(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "uv.lock"), []byte("version = 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	flakePath := filepath.Join(dir, "flake.nix")
	if err := os.WriteFile(flakePath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	generator := newWithRunner(noopRunner{})
	_, err := generator.Generate(context.Background(), req(dir, dir, false))
	if err == nil {
		t.Fatal("expected overwrite error")
	}
}

type noopRunner struct{}

func (noopRunner) ensure(context.Context, string, string, bool) error { return nil }

type copyRunner struct{}

func (copyRunner) ensure(_ context.Context, sourceDir, outputDir string, force bool) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	for _, name := range []string{"pyproject.toml", "uv.lock"} {
		if err := copyProjectFile(filepath.Join(sourceDir, name), filepath.Join(outputDir, name), force); err != nil {
			return err
		}
	}
	return nil
}

func writePyproject(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(`
[project]
name = "demo"
version = "0.1.0"
`), 0644); err != nil {
		t.Fatal(err)
	}
}

func req(dir, outputDir string, force bool) scaffold.Request {
	return scaffold.Request{Dir: dir, OutputDir: outputDir, Force: force}
}
