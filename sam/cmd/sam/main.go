package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "sam",
		Short: "Slop+Me — tmux dev session manager",
	}
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
