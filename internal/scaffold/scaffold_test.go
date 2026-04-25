package scaffold

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestServiceAutoDetectsGenerator(t *testing.T) {
	dir := t.TempDir()
	generator := &fakeGenerator{detect: true}
	service := NewService(RegisteredGenerator{
		Tech:      TechGo,
		Generator: generator,
	})

	result, err := service.Generate(context.Background(), Request{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if result.Tech != TechGo {
		t.Fatalf("tech = %q", result.Tech)
	}
	if generator.request.Dir != dir {
		t.Fatalf("dir = %q", generator.request.Dir)
	}
	if generator.request.OutputDir != dir {
		t.Fatalf("output dir = %q", generator.request.OutputDir)
	}
}

func TestServiceResolvesRelativeOutputDir(t *testing.T) {
	dir := t.TempDir()
	generator := &fakeGenerator{detect: true}
	service := NewService(RegisteredGenerator{
		Tech:      TechGo,
		Generator: generator,
	})

	_, err := service.Generate(context.Background(), Request{
		Dir:       dir,
		OutputDir: "dist",
	})
	if err != nil {
		t.Fatal(err)
	}
	if generator.request.OutputDir != filepath.Join(dir, "dist") {
		t.Fatalf("output dir = %q", generator.request.OutputDir)
	}
}

func TestServiceRejectsUnsupportedTechnology(t *testing.T) {
	_, err := NewService().Generate(context.Background(), Request{
		Dir:  t.TempDir(),
		Tech: "rust",
	})
	if err == nil {
		t.Fatal("expected unsupported technology error")
	}
}

func TestServiceRejectsFileProjectPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(path, []byte("module github.com/acme/demo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := NewService().Generate(context.Background(), Request{Dir: path})
	if err == nil {
		t.Fatal("expected directory error")
	}
}

type fakeGenerator struct {
	detect  bool
	request Request
}

func (g *fakeGenerator) Detect(context.Context, string) (bool, error) {
	return g.detect, nil
}

func (g *fakeGenerator) Generate(_ context.Context, req Request) (Result, error) {
	g.request = req
	return Result{FlakePath: filepath.Join(req.OutputDir, "flake.nix")}, nil
}
