package golang

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BuilderHub/deps2flake/internal/scaffold"
)

func TestDetectGoModule(t *testing.T) {
	dir := t.TempDir()
	generator := NewWithRunner(noopRunner{})

	ok, err := generator.Detect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("empty directory detected as a Go module")
	}

	writeGoMod(t, dir, "module github.com/acme/demo\n\ngo 1.22\n")
	ok, err = generator.Detect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("go.mod was not detected")
	}
}

func TestParseModule(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "module github.com/acme/My_App\n\ngo 1.22\n")

	meta, err := ParseModule(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.ModulePath != "github.com/acme/My_App" {
		t.Fatalf("module path = %q", meta.ModulePath)
	}
	if meta.GoVersion != "1.22" {
		t.Fatalf("go version = %q", meta.GoVersion)
	}
	if meta.PackageName != "my-app" {
		t.Fatalf("package name = %q", meta.PackageName)
	}
}

func TestParseModuleErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := ParseModule(filepath.Join(t.TempDir(), "go.mod"))
		if err == nil {
			t.Fatal("expected missing file error")
		}
		if !strings.Contains(err.Error(), "reading go.mod") {
			t.Fatalf("error does not explain read failure: %v", err)
		}
	})

	t.Run("invalid go mod", func(t *testing.T) {
		dir := t.TempDir()
		writeGoMod(t, dir, "module\n")

		_, err := ParseModule(filepath.Join(dir, "go.mod"))
		if err == nil {
			t.Fatal("expected parse error")
		}
		if !strings.Contains(err.Error(), "parsing go.mod") {
			t.Fatalf("error does not explain parse failure: %v", err)
		}
	})

	t.Run("missing module declaration", func(t *testing.T) {
		dir := t.TempDir()
		writeGoMod(t, dir, "go 1.22\n")

		_, err := ParseModule(filepath.Join(dir, "go.mod"))
		if err == nil {
			t.Fatal("expected missing module error")
		}
		if !strings.Contains(err.Error(), "does not declare a module path") {
			t.Fatalf("error does not explain missing module: %v", err)
		}
	})
}

func TestPackageName(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		want       string
	}{
		{
			name:       "empty",
			modulePath: "",
			want:       "app",
		},
		{
			name:       "symbols only",
			modulePath: "github.com/acme/!!!",
			want:       "app",
		},
		{
			name:       "leading digit",
			modulePath: "github.com/acme/123_service",
			want:       "app-123-service",
		},
		{
			name:       "repeated separators",
			modulePath: "github.com/acme/a___b",
			want:       "a-b",
		},
		{
			name:       "unicode",
			modulePath: "github.com/acme/Über-App",
			want:       "über-app",
		},
		{
			name:       "mixed case",
			modulePath: "github.com/acme/My_App",
			want:       "my-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := packageName(tt.modulePath); got != tt.want {
				t.Fatalf("packageName(%q) = %q, want %q", tt.modulePath, got, tt.want)
			}
		})
	}
}

func TestGenerateRunsNopherAndWritesFlake(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "module github.com/acme/demo\n\ngo 1.22\n")
	writeGoSum(t, dir)

	nopher := fakeNopher(t)
	generator := New(nopher)
	result, err := generator.Generate(context.Background(), scaffold.Request{
		Dir:              dir,
		OutputDir:        dir,
		IncludeContainer: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.FlakePath != filepath.Join(dir, "flake.nix") {
		t.Fatalf("flake path = %q", result.FlakePath)
	}
	if result.LockfilePath != filepath.Join(dir, "nopher.lock.yaml") {
		t.Fatalf("lockfile path = %q", result.LockfilePath)
	}

	flakeData, err := os.ReadFile(result.FlakePath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(flakeData)
	if !strings.Contains(got, "default = app;") {
		t.Fatalf("flake missing default package:\n%s", got)
	}
	if !strings.Contains(got, "container = pkgs.dockerTools.buildLayeredImage") {
		t.Fatalf("flake missing container package:\n%s", got)
	}
	if _, err := os.Stat(result.LockfilePath); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateWritesAllGeneratedFilesToOutputDir(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "dist")
	writeGoMod(t, dir, "module github.com/acme/demo\n\ngo 1.22\n")
	writeGoSum(t, dir)

	result, err := New(fakeNopher(t)).Generate(context.Background(), scaffold.Request{
		Dir:       dir,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.FlakePath != filepath.Join(outputDir, "flake.nix") {
		t.Fatalf("flake path = %q", result.FlakePath)
	}
	if result.LockfilePath != filepath.Join(outputDir, "nopher.lock.yaml") {
		t.Fatalf("lockfile path = %q", result.LockfilePath)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "flake.nix")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "nopher.lock.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "nopher.lock.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source dir lockfile exists or stat failed unexpectedly: %v", err)
	}
}

func TestGenerateChecksOverwriteBeforeRunningNopher(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "module github.com/acme/demo\n\ngo 1.22\n")
	flakePath := filepath.Join(dir, "flake.nix")
	if err := os.WriteFile(flakePath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	runner := &countingRunner{}
	_, err := NewWithRunner(runner).Generate(context.Background(), scaffold.Request{
		Dir:       dir,
		OutputDir: dir,
	})
	if err == nil {
		t.Fatal("expected overwrite error")
	}
	if runner.calls != 0 {
		t.Fatalf("nopher ran before overwrite check: %d calls", runner.calls)
	}
}

func TestCommandRunnerMissingBinary(t *testing.T) {
	dir := t.TempDir()
	err := (CommandRunner{Binary: "deps2flake-nopher-does-not-exist"}).Generate(context.Background(), dir, dir, false)
	if err == nil {
		t.Fatal("expected missing binary error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error does not explain missing binary: %v", err)
	}
}

func TestCommandRunnerIncludesNopherOutputOnFailure(t *testing.T) {
	dir := t.TempDir()
	err := (CommandRunner{Binary: failingNopher(t)}).Generate(context.Background(), dir, dir, false)
	if err == nil {
		t.Fatal("expected nopher failure")
	}
	if !strings.Contains(err.Error(), "running") {
		t.Fatalf("error does not explain command failure: %v", err)
	}
	if !strings.Contains(err.Error(), "boom from nopher") {
		t.Fatalf("error does not include command output: %v", err)
	}
}

func TestCommandRunnerDoesNotOverwriteLockfileInSeparateOutputDir(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := t.TempDir()
	writeGoMod(t, sourceDir, "module github.com/acme/demo\n\ngo 1.22\n")
	writeGoSum(t, sourceDir)

	lockfilePath := filepath.Join(outputDir, "nopher.lock.yaml")
	if err := os.WriteFile(lockfilePath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	err := (CommandRunner{Binary: fakeNopher(t)}).Generate(context.Background(), sourceDir, outputDir, false)
	if err == nil {
		t.Fatal("expected lockfile overwrite error")
	}
	if !strings.Contains(err.Error(), "creating lockfile") {
		t.Fatalf("error does not explain lockfile failure: %v", err)
	}

	data, err := os.ReadFile(lockfilePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing" {
		t.Fatalf("lockfile was overwritten: %q", string(data))
	}
}

type noopRunner struct{}

func (noopRunner) Generate(context.Context, string, string, bool) error {
	return nil
}

type countingRunner struct {
	calls int
}

func (r *countingRunner) Generate(context.Context, string, string, bool) error {
	r.calls++
	return nil
}

func writeGoMod(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeGoSum(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), nil, 0644); err != nil {
		t.Fatal(err)
	}
}

func fakeNopher(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "nopher")
	script := `#!/bin/sh
set -eu
if [ "$1" != "generate" ]; then
  exit 2
fi
touch "$2/nopher.lock.yaml"
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func failingNopher(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "nopher")
	script := `#!/bin/sh
echo "boom from nopher" >&2
exit 7
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}
