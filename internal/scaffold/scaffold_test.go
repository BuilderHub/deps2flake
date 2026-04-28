package scaffold

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestServiceRejectsUndetectedProject(t *testing.T) {
	dir := t.TempDir()
	service := NewService(RegisteredGenerator{
		Tech:      TechGo,
		Generator: &fakeGenerator{},
	})

	_, err := service.Generate(context.Background(), Request{Dir: dir})
	if err == nil {
		t.Fatal("expected detection error")
	}
	if !strings.Contains(err.Error(), "could not detect a supported project type") {
		t.Fatalf("error does not explain detection failure: %v", err)
	}
}

func TestServiceRejectsGoOptionsForNonGoProject(t *testing.T) {
	dir := t.TempDir()
	generator := &fakeGenerator{detect: true}
	service := NewService(RegisteredGenerator{
		Tech:      TechString("python"),
		Generator: generator,
	})

	_, err := service.Generate(context.Background(), Request{
		Dir: dir,
		Go: GoOptions{
			SubPackages: []string{"./cmd/api"},
		},
	})
	if err == nil {
		t.Fatal("expected Go option validation error")
	}
	if !strings.Contains(err.Error(), "go-specific options (--go-*) are only supported for go projects") {
		t.Fatalf("error does not explain Go option validation: %v", err)
	}
	if generator.request.Dir != "" {
		t.Fatalf("generator ran despite invalid Go options: %+v", generator.request)
	}
}

func TestServiceReturnsDetectError(t *testing.T) {
	dir := t.TempDir()
	service := NewService(RegisteredGenerator{
		Tech:      TechGo,
		Generator: &fakeGenerator{detectErr: errors.New("detect failed")},
	})

	_, err := service.Generate(context.Background(), Request{Dir: dir})
	if err == nil {
		t.Fatal("expected detect error")
	}
	if !strings.Contains(err.Error(), "detecting go project") {
		t.Fatalf("error does not include technology context: %v", err)
	}
	if !strings.Contains(err.Error(), "detect failed") {
		t.Fatalf("error does not include original error: %v", err)
	}
}

func TestServiceNormalizesExplicitTechnology(t *testing.T) {
	dir := t.TempDir()
	generator := &fakeGenerator{}
	service := NewService(RegisteredGenerator{
		Tech:      TechGo,
		Generator: generator,
	})

	result, err := service.Generate(context.Background(), Request{
		Dir:  dir,
		Tech: " GO ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Tech != TechGo {
		t.Fatalf("tech = %q", result.Tech)
	}
	if generator.request.Tech != TechGo {
		t.Fatalf("request tech = %q", generator.request.Tech)
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

func TestValidateGoCompiler(t *testing.T) {
	if err := ValidateGoCompiler("pkgs.go"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGoCompiler("pkgs.go_1_24"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGoCompiler(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGoCompiler("foo"); err == nil {
		t.Fatal("expected invalid compiler error")
	}
}

func TestServiceRejectsGoLdflagsForNonGoProject(t *testing.T) {
	dir := t.TempDir()
	generator := &fakeGenerator{detect: true}
	service := NewService(RegisteredGenerator{
		Tech:      TechString("python"),
		Generator: generator,
	})

	_, err := service.Generate(context.Background(), Request{
		Dir: dir,
		Go: GoOptions{
			Ldflags: []string{"-s"},
		},
	})
	if err == nil {
		t.Fatal("expected Go option validation error")
	}
	if !strings.Contains(err.Error(), "go-specific options (--go-*) are only supported for go projects") {
		t.Fatalf("error does not explain Go option validation: %v", err)
	}
}

type fakeGenerator struct {
	detect    bool
	detectErr error
	request   Request
}

func (g *fakeGenerator) Detect(context.Context, string) (bool, error) {
	return g.detect, g.detectErr
}

func (g *fakeGenerator) Generate(_ context.Context, req Request) (Result, error) {
	g.request = req
	return Result{FlakePath: filepath.Join(req.OutputDir, "flake.nix")}, nil
}
