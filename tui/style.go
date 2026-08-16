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

	// seamInk draws the scar, and it is the one thing in this palette that is
	// not rule. A scar was drawn in rule for as long as it existed, which made
	// it the darkest thing on the screen — dimmer than the faded material it
	// stands for (242), dimmer than the header and footer (240) — while being
	// the only object on screen standing for something absent. Measured by ink
	// on real frames, a first-time reader's eye reached it last.
	//
	// 244, not the terminal's own foreground. The finding was the inversion, not
	// the darkness: a claim should not be dimmer than the material it explains.
	// Going all the way to default would put three bright objects into the frame
	// this surface exists to produce — a held row standing between two scars —
	// and the argument of that frame is that there is one.
	//
	// It is a style of its own rather than a brighter rule because rule has three
	// other jobs and two of them break at 244. In a receipt the gutter, the
	// ordinal, the address and the clock are rule while the quoted sentence is
	// dim (240); taking rule up to 244 would draw the machinery brighter than the
	// words it brackets, inverting that block's hierarchy in the act of fixing
	// the scar's. And the edge indicators are full-width runs of dashes, so the
	// same value on them is far more ink than the scar carries and would
	// out-shout the thing this change exists to make findable. Both of those are
	// claims now — an edge only draws at all when something is past it — so this
	// is a judgement about how much ink a claim is worth, not about whether it is
	// one. The record outranks the window onto it.
	//
	// It is brighter than the header and footer at 240, which is deliberate and
	// was looked at rather than reasoned about: chrome is furniture and a scar is
	// the record telling you something is missing.
	seamInk = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	// hot is plain: the terminal's own foreground. Recent material should look
	// like the terminal, not like a theme.
	hot = lipgloss.NewStyle()

	// cooling marks bits that the next fold will absorb. Fading them is the
	// whole point of this UI — you watch heat leave before it goes, and still
	// have time to say something about it.
	cooling = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
)
