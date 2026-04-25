package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCommand(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module github.com/acme/demo\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "go.sum"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{
		"generate",
		projectDir,
		"--nopher-bin",
		fakeNopher(t),
		"--out",
		"dist",
		"--container",
		"--go-package",
		"cmd/api",
		"--go-package",
		"./cmd/worker",
	})

	if err := cmd.Execute(); err != nil {
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
