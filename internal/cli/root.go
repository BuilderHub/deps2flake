// Package cli wires the deps2flake command line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/BuilderHub/deps2flake/internal/golang"
	"github.com/BuilderHub/deps2flake/internal/scaffold"
	"github.com/spf13/cobra"
)

const version = "0.1.0"

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
		Version:       version,
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
					SubPackages: opts.goSubPackages,
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
	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite an existing flake")

	return cmd
}
