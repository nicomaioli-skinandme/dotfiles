package main

import (
	"encoding/json"
	"os"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/workspace"
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
			active, err := workspace.Service{}.Resolve(cfg, workspaceFlag, cwd)
			if err != nil {
				return err
			}
			var name string
			var ws *config.Workspace
			if active != nil {
				name, ws = active.Name, active.WS
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
