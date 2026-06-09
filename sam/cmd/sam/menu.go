package main

import (
	"errors"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/logx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tui"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/wizard"
)

func newMenuCmd(deps tui.Deps) *cobra.Command {
	return &cobra.Command{
		Use:    "menu",
		Short:  "Interactive picker (default when sam is run with no subcommand)",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runMenu(deps, tui.ResWorktrees)
		},
	}
}

// runMenu launches the full-screen TUI on the given starting resource. The
// TUI now attaches to tmux itself (suspending and resuming via
// tea.ExecProcess), so it stays open across an attach and returns here only
// when the user quits or asks to add a workspace. The latter runs the huh
// wizard — an in-process form the TUI can't suspend into — then re-enters
// the menu.
//
// The menu is also the sole launcher of the first-run setup wizard: when no
// config exists yet, it runs the wizard before resolving (the CLI verbs stay
// non-interactive and error instead — see loadWorkspaceAndConfig). A
// cancelled first run leaves no config and exits cleanly.
func runMenu(deps tui.Deps, start tui.Resource) error {
	ready, err := ensureFirstRun()
	if err != nil {
		return err
	}
	if !ready {
		return nil // first-run wizard cancelled — nothing to show
	}

	name, ws, cfg, err := loadWorkspaceAndConfig()
	if err != nil {
		return err
	}

	// When cwd doesn't resolve to a workspace and --workspace wasn't
	// given, open on the workspace-select view so the user picks one
	// explicitly. The TUI will switch m.workspace once they pick.
	if ws == nil {
		start = tui.ResWorkspaces
	}

	// Wire the session logger: a level resolved from flag/env/config, teed
	// to a per-pid temp file and an in-memory ring the `:logs` view reads.
	level := resolveLogLevel(cfg.Log.Level)
	logPath := logx.DefaultPath()
	logger, ring, closeLog := logx.New(level, logPath)
	defer closeLog()
	deps.Logger = logger
	deps.LogRing = ring
	deps.LogPath = logPath

	res, err := tui.Run(name, ws, cfg.Workspaces, start, cfg.Tui, deps)
	if err != nil {
		return err
	}

	if res.RunWizard {
		// The wizard owns the terminal; run it, then drop back into the menu.
		if err := wizard.AddWorkspace(os.Stdout); err != nil {
			return err
		}
		return runMenu(deps, tui.ResWorkspaces)
	}

	return nil // user quit
}

// ensureFirstRun launches the setup wizard when no config exists yet, and
// reports whether a config is present afterward. The menu is the only place
// that runs the wizard; CLI verbs error on a missing config instead (see
// loadWorkspaceAndConfig). A cancelled wizard leaves no file and returns
// false, so the caller exits cleanly without opening the TUI.
func ensureFirstRun() (bool, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := wizard.AddWorkspace(os.Stdout); err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		return false, err
	}
}

// resolveLogLevel picks the minimum log level, preferring the --log-level
// flag, then $SAM_LOG_LEVEL, then the config value (already defaulted by
// config.Load), then DefaultLogLevel. Unparseable flag/env values are
// skipped rather than fatal — logging config should never block the menu.
func resolveLogLevel(cfgLevel string) slog.Level {
	for _, candidate := range []string{logLevelFlag, os.Getenv("SAM_LOG_LEVEL"), cfgLevel, config.DefaultLogLevel} {
		if candidate == "" {
			continue
		}
		if l, err := config.ParseLogLevel(candidate); err == nil {
			return l
		}
	}
	return slog.LevelInfo
}

// shouldDefaultToMenu reports whether `sam` was invoked with no
// subcommand and no top-level help request. --workspace (which carries
// a value, attached or detached) does not count as a subcommand.
func shouldDefaultToMenu(args []string) bool {
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if a == "--" {
			return true
		}
		if a == "--help" || a == "-h" || a == "--version" {
			return false
		}
		if len(a) > 0 && a[0] == '-' {
			// Flags that take a detached value consume the next arg, so it
			// isn't mistaken for a subcommand. Attached forms (--flag=v,
			// -ov) are single args and fall through the continue below.
			if a == "--workspace" || a == "--output" || a == "-o" || a == "--log-level" {
				skipNext = true
			}
			continue
		}
		return false
	}
	return true
}

// maybeDefaultToMenu wires `sam` (no subcommand) to invoke `sam menu`.
func maybeDefaultToMenu(root *cobra.Command) {
	if !shouldDefaultToMenu(os.Args[1:]) {
		return
	}
	newArgs := append([]string{"menu"}, os.Args[1:]...)
	root.SetArgs(newArgs)
}
