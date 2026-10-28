package table

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type StyledString struct {
	open    string
	content string
	close   string
}

const Marker = '\x1B'

func TruncateStyledLines(s string, startIndex, endIndex int) string {
	lines := strings.Split(s, "\n")
	truncatedLines := make([]string, len(lines))
	for i, line := range lines {
		truncatedLines[i] = truncateStyledLine(line, startIndex, endIndex)
	}
	return strings.Join(truncatedLines, "\n")
}

func truncateStyledLine(s string, startIndex, endIndex int) string {
	if len(s) <= startIndex {
		return ""
	}

	spans := parseStyledString(s)
	accountedForLength := 0
	result := ""

	for _, ss := range spans {
		if accountedForLength > endIndex {
			break
		}

		currentWidth := lipgloss.Width(ss.open + ss.content + ss.close)

		if accountedForLength <= startIndex {
			if accountedForLength+currentWidth > startIndex {
				_, right := splitByDsiplayWidth(ss.content, startIndex-accountedForLength)
				result += ss.open + right + ss.close
			}
		} else if accountedForLength+currentWidth >= endIndex {
			// this span gets us to the end
			left, _ := splitByDsiplayWidth(ss.content, endIndex-accountedForLength)
			// preSub := ss.open + leftByDisplayWidth(ss.content, endIndex-accountedForLength) + ss.close
			result += ss.open + left + ss.close
		} else {
			// have not made it to the end yet
			result = result + ss.open + ss.content + ss.close
		}
		accountedForLength += currentWidth
	}

	return result
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

// returns two strings; the first being <= width, the second being anything left over
func splitByDsiplayWidth(s string, width int) (string, string) {
	currentWidth := 0
	var leftResult strings.Builder
	var rightResult strings.Builder

	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if currentWidth+rw > width {
			rightResult.WriteRune(r)
		} else {
			leftResult.WriteRune(r)
		}
		currentWidth += rw
	}
	return leftResult.String(), rightResult.String()
}

func isTerminator(c rune) bool {
	return (c >= 0x40 && c <= 0x5a) || (c >= 0x61 && c <= 0x7a)
}

func ConvertStringRows(rows [][]string) [][]any {
	result := make([][]any, len(rows))
	for i, row := range rows {
		result[i] = make([]any, len(row))
		for j, cell := range row {
			result[i][j] = cell
		}
	}
	return result
}
