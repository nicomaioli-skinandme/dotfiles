package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
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
	root.AddCommand(newListCmd())
	root.AddCommand(newNewWorktreeCmd())
	root.AddCommand(newDeleteCmd())
	root.AddCommand(newMenuCmd())

	maybeDefaultToMenu(root)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "sam:", err)
		os.Exit(1)
	}
}

func loadProject() (*config.Project, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	_, proj, err := config.Resolve(cfg, projectFlag)
	if err != nil {
		return nil, err
	}
	return proj, nil
}
