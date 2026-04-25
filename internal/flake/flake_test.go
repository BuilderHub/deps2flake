package flake

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderGoDefaultPackage(t *testing.T) {
	rendered, err := RenderGo(GoData{
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
		"subPackages = [ \".\" ];",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered flake does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "container =") {
		t.Fatalf("container package rendered without IncludeContainer:\n%s", got)
	}
}

func TestRenderGoContainerPackage(t *testing.T) {
	rendered, err := RenderGo(GoData{
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

func TestRenderGoRequiresPackageName(t *testing.T) {
	_, err := RenderGo(GoData{
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

func TestEnsureWritableRequiresPath(t *testing.T) {
	err := EnsureWritable("", false)
	if err == nil {
		t.Fatal("expected path error")
	}
	if !strings.Contains(err.Error(), "flake path is required") {
		t.Fatalf("error does not explain missing path: %v", err)
	}
}
