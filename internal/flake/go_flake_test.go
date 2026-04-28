package flake

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BuilderHub/deps2flake/internal/scaffold"
)

func TestRenderGoDefaultPackage(t *testing.T) {
	rendered, err := renderGo(GoData{
		PackageName: "demo",
		ModulePath:  "github.com/acme/demo",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := string(rendered)
	for _, want := range []string{
		"nopher.url = \"github:anthr76/nopher\";",
		"default = app;",
		"modules = ./nopher.lock.yaml;",
		"subPackages = [ \"./\" ];",
		`version = "0.1.0";`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered flake does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "container =") {
		t.Fatalf("container package rendered without IncludeContainer:\n%s", got)
	}
}

func TestRenderGoMultipleSubPackages(t *testing.T) {
	rendered, err := renderGo(GoData{
		PackageName: "demo",
		ModulePath:  "github.com/acme/demo",
		GoOptions: scaffold.GoOptions{
			SubPackages: []string{"./cmd/api", "./cmd/worker"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := string(rendered)
	if !strings.Contains(got, "subPackages = [ \"./cmd/api\" \"./cmd/worker\" ];") {
		t.Fatalf("rendered flake does not contain multiple subpackages:\n%s", got)
	}
}

func TestRenderGoContainerPackage(t *testing.T) {
	rendered, err := renderGo(GoData{
		PackageName:      "demo",
		ModulePath:       "github.com/acme/demo",
		IncludeContainer: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	got := string(rendered)
	for _, want := range []string{
		"container = pkgs.dockerTools.buildLayeredImage",
		"Cmd = [ \"${app}/bin/demo\" ];",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered flake does not contain %q:\n%s", want, got)
		}
	}
}

func TestRenderGoNopherPassthrough(t *testing.T) {
	rendered, err := renderGo(GoData{
		PackageName: "demo",
		ModulePath:  "github.com/acme/demo",
		GoOptions: scaffold.GoOptions{
			Ldflags:    []string{"-s", "-w"},
			Tags:       []string{"netgo"},
			CheckFlags: []string{"-short"},
			CGOEnabled: "1",
			GOOS:       "linux",
			GOARCH:     "amd64",
			SkipCheck:  true,
			Compiler:   "pkgs.go_1_24",
			DerivationArgs: []string{
				`buildInputs = [ pkgs.libusb1 ];`,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := string(rendered)
	for _, want := range []string{
		`go = pkgs.go_1_24;`,
		`ldflags = [ "-s" "-w" ];`,
		`tags = [ "netgo" ];`,
		`checkFlags = [ "-short" ];`,
		`CGO_ENABLED = "1";`,
		`GOOS = "linux";`,
		`GOARCH = "amd64";`,
		`doCheck = false;`,
		`buildInputs = [ pkgs.libusb1 ];`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered flake does not contain %q:\n%s", want, got)
		}
	}
}

func TestRenderGoRejectsInvalidCompiler(t *testing.T) {
	_, err := renderGo(GoData{
		PackageName: "demo",
		ModulePath:  "github.com/acme/demo",
		GoOptions: scaffold.GoOptions{
			Compiler: `pkgs; malicious`,
		},
	})
	if err == nil {
		t.Fatal("expected invalid compiler error")
	}
	if !strings.Contains(err.Error(), "go compiler must match") {
		t.Fatalf("error does not explain invalid compiler: %v", err)
	}
}

func TestRenderGoHookEscaping(t *testing.T) {
	rendered, err := renderGo(GoData{
		PackageName: "demo",
		ModulePath:  "github.com/acme/demo",
		GoOptions: scaffold.GoOptions{
			PreBuild: "echo '${app}'\necho it's ok",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(rendered)
	if !strings.Contains(got, "''${app}") {
		t.Fatalf("expected antiquotation escape in preBuild, got:\n%s", got)
	}
	if !strings.Contains(got, "it''s ok") {
		t.Fatalf("expected apostrophe escape in preBuild, got:\n%s", got)
	}
}

func TestNixHookStringEmpty(t *testing.T) {
	if got := nixHookString(""); got != "''" {
		t.Fatalf("nixHookString(\"\") = %q", got)
	}
}

func TestRenderGoRequiresPackageName(t *testing.T) {
	_, err := renderGo(GoData{
		ModulePath: "github.com/acme/demo",
	})
	if err == nil {
		t.Fatal("expected package name error")
	}
	if !strings.Contains(err.Error(), "package name is required") {
		t.Fatalf("error does not explain missing package name: %v", err)
	}
}

func TestWriteGoCreatesNestedOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dist", "flake.nix")
	err := WriteGo(path, GoData{
		PackageName: "demo",
		ModulePath:  "github.com/acme/demo",
	}, false)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "pname = \"demo\";") {
		t.Fatalf("unexpected flake contents:\n%s", string(data))
	}
}

func TestWriteGoDoesNotOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "flake.nix")
	if err := os.WriteFile(path, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	err := WriteGo(path, GoData{
		PackageName: "demo",
		ModulePath:  "github.com/acme/demo",
	}, false)
	if err == nil {
		t.Fatal("expected overwrite error")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "existing" {
		t.Fatalf("file was overwritten: %q", string(data))
	}
}
