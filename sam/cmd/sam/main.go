package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	_ "github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	_ "github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	_ "github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
	_ "github.com/nicomaioli-skinandme/dotfiles/sam/internal/ui"
)

var (
	projectFlag string
	humanFlag   bool
)

func main() {
	root := &cobra.Command{
		Use:           "sam",
		Short:         "Slop+Me — tmux dev session manager",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&projectFlag, "project", "",
		"project name (overrides default_project)")
	root.PersistentFlags().BoolVarP(&humanFlag, "human", "H", false,
		"human-readable output (table) where supported")
	root.AddCommand(newConfigPrintCmd())
	root.AddCommand(newClankersCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "sam:", err)
		os.Exit(1)
	}
}
