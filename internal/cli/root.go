// Package cli wires the deps2flake command line interface.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/BuilderHub/deps2flake/internal/golang"
	"github.com/BuilderHub/deps2flake/internal/scaffold"
	"github.com/BuilderHub/deps2flake/internal/version"
	"github.com/alecthomas/kong"
)

// goGenerator supplies the Go scaffold generator. Tests may replace this for hermetic runs.
var goGenerator = func() scaffold.Generator { return golang.New() }

type rootCLI struct {
	Generate generateCmd `cmd:"" help:"Generate a flake.nix for a project"`
}

type goFlagGroup struct {
	Package       []string `sep:"none" name:"package" help:"Go package to build (repeatable)."`
	Ldflags       []string `sep:"none" help:"Pass -ldflags to go build (repeatable)."`
	Tags          []string `sep:"none" help:"Build tags for go build (repeatable)."`
	CheckFlag     []string `sep:"none" name:"check-flag" help:"Extra args to go test in checkPhase (repeatable)."`
	CGOEnabled    string   `name:"cgo-enabled" help:"Set CGO_ENABLED (e.g. 0 or 1); omit for nopher default."`
	SkipCheck     bool     `name:"skip-check" help:"Set doCheck = false."`
	PreBuild      string   `name:"pre-build" help:"Bash fragment for nopher preBuild."`
	PostBuild     string   `name:"post-build" help:"Bash fragment for nopher postBuild."`
	PreCheck      string   `name:"pre-check" help:"Bash fragment for nopher preCheck."`
	PostCheck     string   `name:"post-check" help:"Bash fragment for nopher postCheck."`
	PreInstall    string   `name:"pre-install" help:"Bash fragment for nopher preInstall."`
	PostInstall   string   `name:"post-install" help:"Bash fragment for nopher postInstall."`
	Compiler      string   `help:"Nix expression for buildNopherGoApp go (e.g. pkgs.go_1_24)."`
	DerivationArg []string `sep:"none" name:"derivation-arg" help:"Raw Nix attr line inside buildNopherGoApp (repeatable)."`
}

type generateCmd struct {
	Path string `arg:"" optional:"" default:"." help:"Project directory."`

	Tech      string `default:"auto" help:"Technology to scaffold: auto or go."`
	Out       string `name:"out" help:"Output directory for generated files (relative to project when not absolute)."`
	Container bool   `name:"container" help:"Also generate packages.container."`
	Force     bool   `help:"Overwrite an existing flake."`

	Go goFlagGroup `embed:"" prefix:"go."`
}

func (g *generateCmd) Run(k *kong.Kong) error {
	if err := scaffold.ValidateGoCompiler(g.Go.Compiler); err != nil {
		return err
	}

	service := scaffold.NewService(scaffold.RegisteredGenerator{
		Tech:      scaffold.TechGo,
		Generator: goGenerator(),
	})
	result, err := service.Generate(context.Background(), scaffold.Request{
		Dir:              g.Path,
		OutputDir:        g.Out,
		Tech:             scaffold.TechString(g.Tech),
		IncludeContainer: g.Container,
		Force:            g.Force,
		Go: scaffold.GoOptions{
			SubPackages:    g.Go.Package,
			Ldflags:        g.Go.Ldflags,
			Tags:           g.Go.Tags,
			CheckFlags:     g.Go.CheckFlag,
			CGOEnabled:     g.Go.CGOEnabled,
			SkipCheck:      g.Go.SkipCheck,
			PreBuild:       g.Go.PreBuild,
			PostBuild:      g.Go.PostBuild,
			PreCheck:       g.Go.PreCheck,
			PostCheck:      g.Go.PostCheck,
			PreInstall:     g.Go.PreInstall,
			PostInstall:    g.Go.PostInstall,
			Compiler:       g.Go.Compiler,
			DerivationArgs: g.Go.DerivationArg,
		},
	})
	if err != nil {
		return err
	}

	out := k.Stdout
	if _, err := fmt.Fprintf(out, "Generated %s project flake: %s\n", result.Tech, result.FlakePath); err != nil {
		return fmt.Errorf("writing command output: %w", err)
	}
	if result.LockfilePath != "" {
		if _, err := fmt.Fprintf(out, "Generated lockfile: %s\n", result.LockfilePath); err != nil {
			return fmt.Errorf("writing command output: %w", err)
		}
	}
	return nil
}

func kongOptions() []kong.Option {
	return []kong.Option{
		kong.Name("deps2flake"),
		kong.Description("Generate Nix flakes from existing dependency files. Use --version or -V to print the version."),
	}
}

// printVersionIfRequested handles --version / -V without a subcommand so Kong tests
// (which cannot use os.Exit) behave like production.
func printVersionIfRequested(args []string, w io.Writer) (handled bool, err error) {
	if len(args) != 1 {
		return false, nil
	}
	switch args[0] {
	case "--version", "-V":
		_, err := fmt.Fprintln(w, version.Version)
		return true, err
	default:
		return false, nil
	}
}

// Execute runs deps2flake and exits non-zero on failure.
func Execute() {
	args := os.Args[1:]
	handled, err := printVersionIfRequested(args, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if handled {
		return
	}
	var cli rootCLI
	parser, err := kong.New(&cli, kongOptions()...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := ctx.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runForTest(args []string, stdout, stderr io.Writer) error {
	handled, err := printVersionIfRequested(args, stdout)
	if err != nil || handled {
		return err
	}
	var cli rootCLI
	parser, err := kong.New(&cli, append([]kong.Option{
		kong.Writers(stdout, stderr),
	}, kongOptions()...)...)
	if err != nil {
		return err
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}
	return ctx.Run()
}
