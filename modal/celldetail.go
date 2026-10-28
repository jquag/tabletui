package modal

import (
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

const hint string = "Yank: y | Close: q or escape"

type CellDetailModalContent struct {
	header string
	value  string
	width  int
	copied bool
}

func NewCellDetailModalContent(header, value string) *CellDetailModalContent {
	width := max(max(len(header), len(value)), len(hint)) + 2
	if width < 20 {
		width = 20
	}
	if width > 60 {
		width = 60
	}

	return &CellDetailModalContent{
		header: header,
		value:  value,
		width:  width,
	}
}

func (c *CellDetailModalContent) View() string {
	wrapped := wordwrap.String(c.value, c.width)
	lines := strings.Split(wrapped, "\n")
	const maxLines = 20
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], "...")
	}
	content := strings.Join(lines, "\n")

	hint := hint
	if c.copied {
		hint = "Yanked!"
	}
	hintStyle := lipgloss.NewStyle().Width(c.width).Foreground(lipgloss.Color("#888888")).Align(lipgloss.Center)

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Width(c.width).PaddingBottom(1).Align(lipgloss.Center).Render(content),
		hintStyle.Render(hint),
	)
}

func (c *CellDetailModalContent) Update(msg tea.Msg) (ModalContent, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			clipboard.WriteAll(c.value)
			c.copied = true
			return c, nil
		case "q":
			return c, CloseCmd
		}
	}
	return c, nil
}

func (c *CellDetailModalContent) ShouldCloseOnEscape() bool {
	return true
}
