package flake

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BuilderHub/deps2flake/internal/scaffold"
)

func TestRenderPythonDefault(t *testing.T) {
	rendered, err := renderPython(PythonData{
		PackageName:    "demo",
		ProjectName:    "demo",
		ProjectVersion: "0.1.0",
		MainProgram:    "demo",
		PythonOptions: scaffold.PythonOptions{
			SourcePreference: "wheel",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := string(rendered)
	for _, want := range []string{
		`url = "github:pyproject-nix/pyproject.nix"`,
		`url = "github:pyproject-nix/uv2nix"`,
		`sourcePreference = "wheel"`,
		`default = app`,
		`program = "${app}/bin/demo"`,
		`mkVirtualEnv "demo-env"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered flake does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "nopher.url") {
		t.Fatalf("python flake must not reference nopher:\n%s", got)
	}
	if strings.Contains(got, "linuxSystem =") {
		t.Fatalf("linuxSystem rendered without IncludeContainer:\n%s", got)
	}
}

func TestRenderPythonWithInterpreterAndHooks(t *testing.T) {
	rendered, err := renderPython(PythonData{
		PackageName:    "demo",
		ProjectName:    "demo",
		ProjectVersion: "1.0.0",
		MainProgram:    "demo-cli",
		PythonOptions: scaffold.PythonOptions{
			Interpreter:      "pkgs.python312",
			SourcePreference: "sdist",
			SkipCheck:        true,
			PreBuild:         "echo pre",
			CheckFlags:       []string{"-q"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := string(rendered)
	for _, want := range []string{
		`pkgs.python312`,
		`sourcePreference = "sdist"`,
		`doCheck = false`,
		`preBuild =`,
		`echo pre`,
		`checkFlags = [ "-q" ]`,
		`stdenv.mkDerivation`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered flake does not contain %q:\n%s", want, got)
		}
	}
}

func TestRenderPythonContainer(t *testing.T) {
	rendered, err := renderPython(PythonData{
		PackageName:      "demo",
		ProjectName:      "demo",
		ProjectVersion:   "0.1.0",
		MainProgram:      "demo",
		IncludeContainer: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := string(rendered)
	if !strings.Contains(got, "container = pkgs.dockerTools.buildLayeredImage") {
		t.Fatalf("missing container package:\n%s", got)
	}
}

func TestRenderPythonRejectsInvalidInterpreter(t *testing.T) {
	_, err := renderPython(PythonData{
		PackageName: "demo",
		ProjectName: "demo",
		PythonOptions: scaffold.PythonOptions{
			Interpreter: "notvalid",
		},
	})
	if err == nil {
		t.Fatal("expected invalid interpreter error")
	}
}

func TestWritePythonCreatesNestedOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dist", "flake.nix")
	err := WritePython(path, PythonData{
		PackageName:    "demo",
		ProjectName:    "demo",
		ProjectVersion: "0.1.0",
		MainProgram:    "demo",
	}, false)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `pname = "demo"`) && !strings.Contains(string(data), "demo-env") {
		t.Fatalf("unexpected flake:\n%s", string(data))
	}
}
