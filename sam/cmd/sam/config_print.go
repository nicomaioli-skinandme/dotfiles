package main

import (
	"encoding/json"
	"os"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/spf13/cobra"
)

func newConfigPrintCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "config-print",
		Short:  "Print the resolved sam config (debug)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			cwd, _ := os.Getwd()
			name, proj, err := config.Resolve(cfg, projectFlag, cwd)
			if err != nil {
				return err
			}

			out := struct {
				Path    string          `json:"path"`
				Project string          `json:"project"`
				Config  *config.Project `json:"config"`
			}{path, name, proj}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
}
