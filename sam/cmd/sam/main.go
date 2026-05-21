package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	_ "github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	_ "github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	_ "github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ui"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/wizard"
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
	root.AddCommand(newFromIssueCmd())
	root.AddCommand(newMenuCmd())
	root.AddCommand(newProjectCmd())

	maybeDefaultToMenu(root)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "sam:", err)
		os.Exit(1)
	}
}

// loadProject resolves the active project for the current invocation.
// Returns (name, *Project) so callers that need the project name (e.g.
// the worktree-setup hook) don't have to look it up again.
//
// First-run side effect: when ~/.config/sam/config.toml is missing,
// runs the setup wizard, saves the result, and re-execs `sam` with
// no arguments so the user lands in a clean menu. This call does not
// return in that case.
func loadProject() (string, *config.Project, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return "", nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := runFirstRunWizard(path); err != nil {
			return "", nil, err
		}
		// runFirstRunWizard either re-execs (no return) or returns
		// nil after the user cancelled, in which case we exit.
		os.Exit(0)
	} else if err != nil {
		return "", nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return "", nil, err
	}
	cwd, _ := os.Getwd()
	name, proj, err := config.Resolve(cfg, projectFlag, cwd)
	if err != nil {
		return "", nil, err
	}
	return name, proj, nil
}

func runFirstRunWizard(path string) error {
	fmt.Fprintln(os.Stderr, "sam: no config at "+path+" — launching setup wizard.")
	cfg, err := wizard.Run(nil)
	if err != nil {
		if errors.Is(err, ui.ErrCancelled) {
			return nil
		}
		return err
	}
	if err := config.Save(cfg, path); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Wrote "+path)
	// Re-exec `sam` with no args so the user lands in a clean menu,
	// as if they had just invoked sam fresh. Whatever they originally
	// typed (e.g. `sam from-issue`) is intentionally discarded.
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(bin, []string{"sam"}, os.Environ())
}
