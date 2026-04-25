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
