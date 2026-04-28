package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BuilderHub/deps2flake/internal/version"
)

func TestCLIVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runForTest([]string{"--version"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), version.Version) {
		t.Fatalf("expected version in stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func TestGenerateCommand(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module github.com/acme/demo\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "go.sum"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	err := runForTest([]string{
		"generate",
		projectDir,
		"--nopher-bin",
		fakeNopher(t),
		"--out",
		"dist",
		"--container",
		"--go.package",
		"cmd/api",
		"--go.package",
		"./cmd/worker",
		"--go.tags",
		"netgo",
		"--go.ldflags=-s",
	}, &output, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(projectDir, "dist", "flake.nix")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "dist", "nopher.lock.yaml")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Generated go project flake") {
		t.Fatalf("unexpected output: %q", output.String())
	}
	flakeData, err := os.ReadFile(filepath.Join(projectDir, "dist", "flake.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(flakeData), `subPackages = [ "./cmd/api" "./cmd/worker" ];`) {
		t.Fatalf("flake missing normalized Go packages:\n%s", string(flakeData))
	}
	if !strings.Contains(string(flakeData), `tags = [ "netgo" ];`) {
		t.Fatalf("flake missing build tags:\n%s", string(flakeData))
	}
	if !strings.Contains(string(flakeData), `ldflags = [ "-s" ];`) {
		t.Fatalf("flake missing ldflags:\n%s", string(flakeData))
	}
}

func TestGenerateCommandRejectsInvalidGoCompiler(t *testing.T) {
	err := runForTest([]string{
		"generate",
		t.TempDir(),
		"--go.compiler",
		"notvalid",
	}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected invalid go compiler error")
	}
	if !strings.Contains(err.Error(), "go compiler must match") {
		t.Fatalf("error does not explain invalid compiler: %v", err)
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
