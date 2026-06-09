package tui

import (
	"charm.land/lipgloss/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
)

// styles holds every lipgloss style the menu renders with, built once from
// the configured palette. Keeping them on the model (rather than as package
// globals) is what makes the palette configurable.
//
// The palette is deliberately small (see issue #24): primary for accents and
// highlights, secondary for chrome, destroy for destructive affordances, and
// the terminal default for body text. Many former styles that each had their
// own color now share one of these three.
type styles struct {
	divider      lipgloss.Style // list/section rules
	hint         lipgloss.Style // faint help text
	cursor       lipgloss.Style // the ▸ row pointer + its title
	selected     lipgloss.Style // multi-select ✓
	row          lipgloss.Style // an unselected row title
	active       lipgloss.Style // running-session ● bullet
	detail       lipgloss.Style // trailing "(detail)" text
	breadcrumb   lipgloss.Style // status-bar workspace › scope
	statusInfo   lipgloss.Style // transient status message
	modalBorder  lipgloss.Style // dialog frame
	modalAffirm  lipgloss.Style // an inactive Yes/No button
	modalActive  lipgloss.Style // the highlighted button (non-destructive)
	modalDestroy lipgloss.Style // the highlighted button on a destructive confirm
	deleting     lipgloss.Style // the "deleting…" row indicator

	// Autocomplete popup styles, copied onto the autocomplete struct.
	acSelected lipgloss.Style // the cursor row
	acBorder   lipgloss.Style // popup frame
}

// newStyles assembles the palette-derived styles. Primary/Secondary/Destroy
// are always set (config.Load fills defaults); Foreground/Background are
// applied only when non-empty so an unset value means the terminal default.
func newStyles(c config.Colors) styles {
	primary := lipgloss.Color(c.Primary)
	secondary := lipgloss.Color(c.Secondary)
	destroy := lipgloss.Color(c.Destroy)

	// base carries any configured foreground/background; left untouched it
	// renders in the terminal's own colors.
	base := lipgloss.NewStyle()
	if c.Foreground != "" {
		base = base.Foreground(lipgloss.Color(c.Foreground))
	}
	if c.Background != "" {
		base = base.Background(lipgloss.Color(c.Background))
	}

	return styles{
		divider:    base.Foreground(secondary),
		hint:       base.Foreground(secondary),
		cursor:     base.Foreground(primary).Bold(true),
		selected:   base.Foreground(primary),
		row:        base,
		active:     base.Foreground(primary),
		detail:     base.Foreground(secondary),
		breadcrumb: base.Foreground(primary).Bold(true),
		statusInfo: base.Foreground(primary),
		modalBorder: base.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primary).
			Padding(1, 3),
		modalAffirm:  base.Padding(0, 2),
		modalActive:  base.Padding(0, 2).Reverse(true).Bold(true),
		modalDestroy: base.Padding(0, 2).Foreground(destroy).Reverse(true).Bold(true),
		deleting:     base.Foreground(destroy),
		acSelected:   base.Reverse(true),
		acBorder: base.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primary).
			Padding(0, 1),
	}
}
