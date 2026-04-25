package golang

import (
	"context"
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

func TestGenerateRunsNopherAndWritesFlake(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "module github.com/acme/demo\n\ngo 1.22\n")

	nopher := fakeNopher(t)
	generator := New(nopher)
	result, err := generator.Generate(context.Background(), scaffold.Request{
		Dir:              dir,
		OutputPath:       filepath.Join(dir, "flake.nix"),
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

func TestGenerateChecksOverwriteBeforeRunningNopher(t *testing.T) {
	dir := t.TempDir()
	writeGoMod(t, dir, "module github.com/acme/demo\n\ngo 1.22\n")
	flakePath := filepath.Join(dir, "flake.nix")
	if err := os.WriteFile(flakePath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	runner := &countingRunner{}
	_, err := NewWithRunner(runner).Generate(context.Background(), scaffold.Request{
		Dir:        dir,
		OutputPath: flakePath,
	})
	if err == nil {
		t.Fatal("expected overwrite error")
	}
	if runner.calls != 0 {
		t.Fatalf("nopher ran before overwrite check: %d calls", runner.calls)
	}
}

func TestCommandRunnerMissingBinary(t *testing.T) {
	err := (CommandRunner{Binary: "deps2flake-nopher-does-not-exist"}).Generate(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("expected missing binary error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error does not explain missing binary: %v", err)
	}
}

type noopRunner struct{}

func (noopRunner) Generate(context.Context, string) error {
	return nil
}

type countingRunner struct {
	calls int
}

func (r *countingRunner) Generate(context.Context, string) error {
	r.calls++
	return nil
}

func writeGoMod(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(contents), 0644); err != nil {
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
