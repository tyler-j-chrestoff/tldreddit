package tui

import "charm.land/lipgloss/v2"

// The palette is deliberately small and ANSI-indexed, so it inherits whatever
// theme the user's terminal already has instead of fighting it.
var (
	dim     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	rule    = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	speaker = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	system  = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	warm    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	// hot is plain: the terminal's own foreground. Recent material should look
	// like the terminal, not like a theme.
	hot = lipgloss.NewStyle()

	// cooling marks bits that the next fold will absorb. Fading them is the
	// whole point of this UI — you watch heat leave before it goes, and still
	// have time to say something about it.
	cooling = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
)
