// Package scaffold coordinates technology-specific flake generation.
package scaffold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TechString identifies a supported project technology.
type TechString string

const (
	TechAuto TechString = "auto"
	TechGo   TechString = "go"
)

// Request contains the user-facing options for generating a project flake.
type Request struct {
	Dir              string
	OutputDir        string
	Tech             TechString
	IncludeContainer bool
	Force            bool
	Go               GoOptions
}

// GoOptions contains Go-specific scaffold options.
type GoOptions struct {
	SubPackages []string
}

// Result describes the files produced by a generator.
type Result struct {
	Tech         TechString
	FlakePath    string
	LockfilePath string
}

// Generator describes a technology-specific project scaffold generator.
type Generator interface {
	Detect(ctx context.Context, dir string) (bool, error)
	Generate(ctx context.Context, req Request) (Result, error)
}

// RegisteredGenerator binds a generator implementation to a technology name.
type RegisteredGenerator struct {
	Tech      TechString
	Generator Generator
}

// Service chooses a technology generator and delegates scaffold generation.
type Service struct {
	generators []RegisteredGenerator
}

// NewService creates a scaffold service with the supplied generators.
func NewService(generators ...RegisteredGenerator) *Service {
	return &Service{generators: generators}
}

// Generate detects or selects a technology generator, then generates the flake.
func (s *Service) Generate(ctx context.Context, req Request) (Result, error) {
	req.Tech = normalizeTech(req.Tech)
	if err := validateGoOptionsForTech(req.Go, req.Tech); err != nil {
		return Result{}, err
	}
	if req.Dir == "" {
		req.Dir = "."
	}

	dir, err := filepath.Abs(req.Dir)
	if err != nil {
		return Result{}, fmt.Errorf("resolving project directory: %w", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return Result{}, fmt.Errorf("reading project directory: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("project path %q is not a directory", dir)
	}
	req.Dir = dir

	if req.OutputDir == "" {
		req.OutputDir = dir
	} else if !filepath.IsAbs(req.OutputDir) {
		req.OutputDir = filepath.Join(dir, req.OutputDir)
	}

	generator, tech, err := s.generatorFor(ctx, req.Tech, dir)
	if err != nil {
		return Result{}, err
	}
	if err := validateGoOptionsForTech(req.Go, tech); err != nil {
		return Result{}, err
	}

	result, err := generator.Generate(ctx, req)
	if err != nil {
		return Result{}, err
	}
	result.Tech = tech
	return result, nil
}

func (s *Service) generatorFor(ctx context.Context, tech TechString, dir string) (Generator, TechString, error) {
	if tech != TechAuto {
		for _, registered := range s.generators {
			if registered.Tech == tech {
				return registered.Generator, registered.Tech, nil
			}
		}
		return nil, "", fmt.Errorf("unsupported technology %q", tech)
	}

	for _, registered := range s.generators {
		ok, err := registered.Generator.Detect(ctx, dir)
		if err != nil {
			return nil, "", fmt.Errorf("detecting %s project: %w", registered.Tech, err)
		}
		if ok {
			return registered.Generator, registered.Tech, nil
		}
	}

	return nil, "", fmt.Errorf("could not detect a supported project type in %q", dir)
}

func validateGoOptionsForTech(opts GoOptions, tech TechString) error {
	if len(opts.SubPackages) == 0 || tech == TechAuto || tech == TechGo {
		return nil
	}
	return fmt.Errorf("--go-package is only supported for go projects")
}

func normalizeTech(tech TechString) TechString {
	switch strings.TrimSpace(strings.ToLower(string(tech))) {
	case "":
		return TechAuto
	default:
		return TechString(strings.TrimSpace(strings.ToLower(string(tech))))
	}
}
