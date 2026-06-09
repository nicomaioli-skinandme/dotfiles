// Package cli is the config View: the `sam config print` (hidden, debug) and
// `sam config doctor` commands. config is infra (no Controller); print dumps
// the resolved config as indented JSON regardless of --output, so the loaders
// are injected by cmd/sam (a lenient resolve that leaves the workspace empty
// rather than erroring when cwd is ambiguous). doctor resolves the config
// path itself and reports problems.
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config/doctor"
)

type (
	// LoadConfig returns the loaded config and its path.
	LoadConfig = func() (*config.Config, string, error)
	// Resolve returns the active workspace, leaving it nil/"" when cwd is
	// ambiguous (it does not turn that into an error).
	Resolve = func() (*config.Workspace, string, error)
)

// NewCmd builds the `config` noun command with its hidden print verb.
func NewCmd(load LoadConfig, resolve Resolve) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect sam configuration",
	}
	cmd.AddCommand(newPrintCmd(load, resolve))
	cmd.AddCommand(newDoctorCmd())
	return cmd
}

// newDoctorCmd builds `sam config doctor`: it validates the active config and
// lists every problem found without mutating anything, exiting non-zero when
// any are present. It resolves the config path itself (no injected loader).
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Report problems with the active config (non-zero exit if any)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			rep := doctor.Run(path)
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "config: %s\n", rep.Path)
			if rep.OK() {
				fmt.Fprintln(out, "✓ no problems found")
				return nil
			}
			for _, issue := range rep.Issues {
				fmt.Fprintf(out, "✗ %s\n", issue)
			}
			return fmt.Errorf("%d problem(s) found", len(rep.Issues))
		},
	}
}

func newPrintCmd(load LoadConfig, resolve Resolve) *cobra.Command {
	return &cobra.Command{
		Use:    "print",
		Short:  "Print the resolved sam config (debug)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, path, err := load()
			if err != nil {
				return err
			}
			ws, name, err := resolve()
			if err != nil {
				return err
			}
			out := struct {
				Path      string            `json:"path"`
				Workspace string            `json:"workspace"`
				Config    *config.Workspace `json:"config"`
			}{path, name, ws}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
}
