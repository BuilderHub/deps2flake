package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BuilderHub/deps2flake/internal/golang"
	"github.com/BuilderHub/deps2flake/internal/scaffold"
	"github.com/BuilderHub/deps2flake/internal/version"
	nophergen "github.com/anthr76/nopher/pkg/generator"
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
	prev := goGenerator
	t.Cleanup(func() { goGenerator = prev })
	goGenerator = func() scaffold.Generator {
		return golang.NewWithNopherOptions(testNopherOptions())
	}

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

func testNopherOptions() nophergen.Options {
	return nophergen.Options{
		Fetch: func(modulePath, version string) (*nophergen.FetchResult, error) {
			return &nophergen.FetchResult{
				Hash: "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
				URL:  "https://example.invalid/" + modulePath + "/@v/" + version + ".zip",
				Rev:  "0000000000000000000000000000000000000000",
			}, nil
		},
	}
}
