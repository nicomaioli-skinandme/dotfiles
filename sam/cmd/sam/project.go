package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ui"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/wizard"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage sam project configuration",
	}
	cmd.AddCommand(newProjectAddCmd())
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a project to ~/.config/sam/config.toml via guided wizard",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runProjectAdd(cmd.OutOrStdout())
		},
	}
}

func runProjectAdd(out interface{ Write([]byte) (int, error) }) error {
	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	var existing *config.Config
	if _, err := os.Stat(path); err == nil {
		existing, err = config.Load(path)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	updated, err := wizard.Run(existing)
	if err != nil {
		if errors.Is(err, ui.ErrCancelled) {
			return nil
		}
		return err
	}
	if err := config.Save(updated, path); err != nil {
		return err
	}
	fmt.Fprintf(out, "Wrote %s\n", path)
	return nil
}
