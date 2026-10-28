package modal

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/padding"
)

const Marker = '\x1B'

func isTerminator(c rune) bool {
	return (c >= 0x40 && c <= 0x5a) || (c >= 0x61 && c <= 0x7a)
}

type StyledString struct {
	open    string
	content string
	close   string
}

type ModalContent interface {
	View() string
	Update(msg tea.Msg) (ModalContent, tea.Cmd)
	ShouldCloseOnEscape() bool
}

type Model struct {
	Width          int
	Height         int
	Content        ModalContent
	BackgroundView string
	Show           bool
	BorderColor    string
	Title          string
	// Position overrides. When >= 0, the modal is placed at that
	// coordinate instead of being centered along that axis.
	// Default -1 means center.
	X        int
	Y        int
	NoBorder bool
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case tea.KeyEscape.String():
			if m.Content.ShouldCloseOnEscape() {
				return m, CloseCmd
			}
		}
	}

	if m.Show {
		newContent, cmd := m.Content.Update(msg)
		m.Content = newContent
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if !m.Show {
		return m.BackgroundView
	}
	if m.Width == 0 || m.Height == 0 {
		return m.BackgroundView
	}

	var modal string
	if m.NoBorder {
		modal = m.Content.View()
	} else {
		modalBoxStyle := lipgloss.NewStyle().Padding(1, 1, 0, 1)
		if m.Title == "" {
			modalBoxStyle = modalBoxStyle.Border(lipgloss.NormalBorder(), true).BorderForeground(m.borderColor())
		} else {
			modalBoxStyle = modalBoxStyle.Inherit(titledBorderStyle(m.borderColor(), m.Title, m.Width))
		}
		modal = modalBoxStyle.Render(m.Content.View())
	}

	modalWidth, modalHeight := lipgloss.Size(modal)

	var startX, startY int
	if m.X >= 0 {
		startX = m.X
	} else {
		startX = max(0, (m.Width/2)-(modalWidth/2))
	}
	if m.Y >= 0 {
		startY = m.Y
	} else {
		startY = max(0, (m.Height/2)-(modalHeight/2))
	}

	lines := strings.Split(m.BackgroundView, "\n")
	updatedLines := []string{}
	modalLines := strings.Split(modal, "\n")

	for i, line := range lines {
		if len(modalLines) > i-startY && i >= startY {
			replaced := replaceChunk(line, startX, modalLines[i-startY])
			updatedLines = append(updatedLines, replaced)
		} else {
			updatedLines = append(updatedLines, line)
		}
	}

	return strings.Join(updatedLines, "\n")
}

func (m Model) borderColor() lipgloss.Color {
	if m.BorderColor != "" {
		return lipgloss.Color(m.BorderColor)
	}
	return lipgloss.Color("#00ff00")
}

func replaceChunk(s string, startIndex int, replacement string) string {
	if s == "" {
		s = strings.Repeat(" ", startIndex+1)
	} else {
		s = padding.String(s, uint(startIndex))
	}

	spans := parseStyledString(s)
	replaced := ""

	replacementWidth := lipgloss.Width(replacement)
	accountedForLength := 0
	for _, ss := range spans {
		currentWidth := lipgloss.Width(ss.open + ss.content + ss.close)

		if accountedForLength > startIndex {
			// past the start of the modal
			if accountedForLength > startIndex+replacementWidth {
				// past the end of the modal
				replaced = replaced + ss.open + ss.content + ss.close
			} else if accountedForLength+currentWidth > startIndex+replacementWidth {
				// this span goes past the end of the modal
				replaced += replacement + ss.open + rightByDisplayWidth(ss.content, startIndex-accountedForLength+replacementWidth) + ss.close
			}
		} else if accountedForLength+currentWidth >= startIndex {
			// this span gets us to the modal
			preSub := ss.open + leftByDisplayWidth(ss.content, startIndex-accountedForLength) + ss.close
			replaced += preSub
			if accountedForLength+currentWidth >= startIndex+replacementWidth {
				replaced = replaced + replacement + ss.open + string([]rune(ss.content)[startIndex-accountedForLength+replacementWidth:]) + ss.close
			}
		} else {
			// have not made it to the modal yet
			replaced = replaced + ss.open + ss.content + ss.close
		}
		accountedForLength += currentWidth
	}

	if lipgloss.Width(replaced) <= startIndex {
		replaced += replacement
	}

	return replaced
}

func leftByDisplayWidth(s string, width int) string {
	currentWidth := 0
	var result strings.Builder
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if currentWidth+rw > width {
			break
		}
		result.WriteRune(r)
		currentWidth += rw
	}
	return result.String()
}

func rightByDisplayWidth(s string, width int) string {
	currentWidth := 0
	var result strings.Builder
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if currentWidth < width {
			currentWidth += rw
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

func parseStyledString(s string) []StyledString {
	spans := []StyledString{}

	current := StyledString{}
	ansiOpening := false
	ansiClosing := false
	for _, c := range s {
		if c == Marker {
			if current.open == "" {
				if current.content != "" {
					spans = append(spans, current)
					current = StyledString{}
				}
				ansiOpening = true
				current.open += string(c)
			} else {
				ansiClosing = true
				current.close += string(c)
			}
		} else if ansiOpening {
			current.open += string(c)
			if isTerminator(c) {
				ansiOpening = false
			}
		} else if ansiClosing {
			current.close += string(c)
			if isTerminator(c) {
				ansiClosing = false
				spans = append(spans, current)
				current = StyledString{}
			}
		} else {
			current.content += string(c)
		}
	}

	if current.close == "" && current.content != "" {
		spans = append(spans, current)
	}

	return spans
}

type CloseMsg int

const close CloseMsg = 1

func CloseCmd() tea.Msg {
	return close
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func titledBorderStyle(color lipgloss.Color, title string, width int) lipgloss.Style {
	style := lipgloss.NewStyle()
	if width <= 0 {
		return style
	}

	topLength := width + 2
	title = title[:min(len(title), topLength)]

	top := fmt.Sprintf("─%s%s", title, strings.Repeat("─", max(0, topLength-len(title)-1)))

	var borderWithTitle = lipgloss.Border{
		Top:          top,
		Bottom:       "─",
		Left:         "│",
		Right:        "│",
		TopLeft:      "╭",
		TopRight:     "╮",
		BottomLeft:   "╰",
		BottomRight:  "╯",
		MiddleLeft:   "├",
		MiddleRight:  "┤",
		Middle:       "┼",
		MiddleTop:    "┬",
		MiddleBottom: "┴",
	}
	return style.BorderStyle(borderWithTitle).BorderForeground(color)
}
