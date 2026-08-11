// Command tldr is the tldreddit terminal client.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/tyler-j-chrestoff/tldreddit/tui"
)

func main() {
	if _, err := tea.NewProgram(tui.New()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tldr: %v\n", err)
		os.Exit(1)
	}
}
