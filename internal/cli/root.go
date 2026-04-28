// Package cli wires the deps2flake command line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/BuilderHub/deps2flake/internal/golang"
	"github.com/BuilderHub/deps2flake/internal/scaffold"
	"github.com/BuilderHub/deps2flake/internal/version"
	"github.com/spf13/cobra"
)

// Execute runs the root command and exits non-zero on failure.
func Execute() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "deps2flake",
		Short:         "Generate Nix flakes from existing dependency files",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.AddCommand(newGenerateCommand())
	return cmd
}

type generateOptions struct {
	tech             string
	includeContainer bool
	outputDir        string
	nopherBin        string
	goSubPackages    []string
	goLdflags        []string
	goTags           []string
	goCheckFlags     []string
	goCGOEnabled     string
	goOS             string
	goArch           string
	goSkipCheck      bool
	goPreBuild       string
	goPostBuild      string
	goPreCheck       string
	goPostCheck      string
	goPreInstall     string
	goPostInstall    string
	goCompiler       string
	goDerivationArg  []string
	force            bool
}

func newGenerateCommand() *cobra.Command {
	opts := generateOptions{
		tech:      string(scaffold.TechAuto),
		nopherBin: "nopher",
	}

	cmd := &cobra.Command{
		Use:   "generate [path]",
		Short: "Generate a flake.nix for a project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}

			if err := scaffold.ValidateGoCompiler(opts.goCompiler); err != nil {
				return err
			}

			service := scaffold.NewService(scaffold.RegisteredGenerator{
				Tech:      scaffold.TechGo,
				Generator: golang.New(opts.nopherBin),
			})
			result, err := service.Generate(cmd.Context(), scaffold.Request{
				Dir:              dir,
				OutputDir:        opts.outputDir,
				Tech:             scaffold.TechString(opts.tech),
				IncludeContainer: opts.includeContainer,
				Force:            opts.force,
				Go: scaffold.GoOptions{
					SubPackages:    opts.goSubPackages,
					Ldflags:        opts.goLdflags,
					Tags:           opts.goTags,
					CheckFlags:     opts.goCheckFlags,
					CGOEnabled:     opts.goCGOEnabled,
					GOOS:           opts.goOS,
					GOARCH:         opts.goArch,
					SkipCheck:      opts.goSkipCheck,
					PreBuild:       opts.goPreBuild,
					PostBuild:      opts.goPostBuild,
					PreCheck:       opts.goPreCheck,
					PostCheck:      opts.goPostCheck,
					PreInstall:     opts.goPreInstall,
					PostInstall:    opts.goPostInstall,
					Compiler:       opts.goCompiler,
					DerivationArgs: opts.goDerivationArg,
				},
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if _, err := fmt.Fprintf(out, "Generated %s project flake: %s\n", result.Tech, result.FlakePath); err != nil {
				return fmt.Errorf("writing command output: %w", err)
			}
			if result.LockfilePath != "" {
				if _, err := fmt.Fprintf(out, "Generated lockfile: %s\n", result.LockfilePath); err != nil {
					return fmt.Errorf("writing command output: %w", err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.tech, "tech", opts.tech, "technology to scaffold: auto or go")
	cmd.Flags().BoolVar(&opts.includeContainer, "container", false, "also generate packages.container")
	cmd.Flags().StringVar(&opts.outputDir, "out", "", "output directory for generated files, relative to the project directory when not absolute")
	cmd.Flags().StringVar(&opts.nopherBin, "nopher-bin", opts.nopherBin, "nopher executable to run for Go projects")
	cmd.Flags().StringArrayVar(&opts.goSubPackages, "go-package", nil, "Go package to build; repeat to include multiple subpackages")
	cmd.Flags().StringArrayVar(&opts.goLdflags, "go-ldflags", nil, "pass -ldflags to go build (repeatable)")
	cmd.Flags().StringArrayVar(&opts.goTags, "go-tags", nil, "build tags for go build (repeatable)")
	cmd.Flags().StringArrayVar(&opts.goCheckFlags, "go-check-flag", nil, "extra args to go test in checkPhase (repeatable)")
	cmd.Flags().StringVar(&opts.goCGOEnabled, "go-cgo-enabled", "", "set CGO_ENABLED for the build (e.g. 0 or 1); omit to use nopher default")
	cmd.Flags().StringVar(&opts.goOS, "go-os", "", "set GOOS for the build; omit for nopher default (null)")
	cmd.Flags().StringVar(&opts.goArch, "go-arch", "", "set GOARCH for the build; omit for nopher default (null)")
	cmd.Flags().BoolVar(&opts.goSkipCheck, "go-skip-check", false, "set doCheck = false")
	cmd.Flags().StringVar(&opts.goPreBuild, "go-pre-build", "", "bash fragment passed to nopher preBuild")
	cmd.Flags().StringVar(&opts.goPostBuild, "go-post-build", "", "bash fragment passed to nopher postBuild")
	cmd.Flags().StringVar(&opts.goPreCheck, "go-pre-check", "", "bash fragment passed to nopher preCheck")
	cmd.Flags().StringVar(&opts.goPostCheck, "go-post-check", "", "bash fragment passed to nopher postCheck")
	cmd.Flags().StringVar(&opts.goPreInstall, "go-pre-install", "", "bash fragment passed to nopher preInstall")
	cmd.Flags().StringVar(&opts.goPostInstall, "go-post-install", "", "bash fragment passed to nopher postInstall")
	cmd.Flags().StringVar(&opts.goCompiler, "go-compiler", "", "Nix expression for buildNopherGoApp go (e.g. pkgs.go_1_24)")
	cmd.Flags().StringArrayVar(&opts.goDerivationArg, "go-derivation-arg", nil, "raw Nix attr line inside buildNopherGoApp (repeatable; advanced)")
	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite an existing flake")

	return cmd
}
