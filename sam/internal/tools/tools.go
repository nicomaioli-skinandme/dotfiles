//go:build tools

// Anchors module deps not yet used by runtime code so `go mod tidy` keeps
// them. Removed once real code imports them.
package tools

import (
	_ "github.com/charmbracelet/huh"
	_ "github.com/spf13/viper"
)
