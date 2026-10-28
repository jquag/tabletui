package table

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CellRef struct {
	Row int
	Col int
}

type Model struct {
	Header        []string
	Data          [][]any
	Focus         bool
	SelectedCell  CellRef
	width         int
	height        int
	colWidths     []int
	viewport      *viewport.Model
	hScrollOffset int
}

var (
	borderColor       = lipgloss.Color("#999999")
	headerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#89b5f9"))
	dataStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#ced6f3"))
	selectedDataStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ced6f3")).Background(lipgloss.Color("#8f617a"))
)

func (m *Model) Init() tea.Cmd {
	return nil
}

func truncateString(s string, width int) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", "")
	if len(s) <= width {
		return s + strings.Repeat(" ", width-len(s))
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "…"
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.Focus {
			return m, nil
		}

		switch msg.String() {
		case "h", "left":
			m.moveLeft(false)
			m.ensureSelectedCellVisible()
			return m, nil
		case "l", "right":
			m.moveRight(false)
			m.ensureSelectedCellVisible()
			return m, nil
		case "tab":
			m.moveRight(true)
			m.ensureSelectedCellVisible()
			return m, nil
		case "shift+tab":
			m.moveLeft(true)
			m.ensureSelectedCellVisible()
			return m, nil
		case "j", "down":
			m.moveDown()
			m.ensureSelectedCellVisible()
			return m, nil
		case "k", "up":
			m.moveUp()
			m.ensureSelectedCellVisible()
			return m, nil
		default:
			_, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	return m, cmd
}

func (m *Model) moveRight(nextRowOnEnd bool) {
	maxCol := len(m.Header) - 1
	if m.SelectedCell.Row < len(m.Data) && len(m.Data[m.SelectedCell.Row]) > 0 {
		maxCol = min(maxCol, len(m.Data[m.SelectedCell.Row])-1)
	}
	if m.SelectedCell.Col < maxCol {
		m.SelectedCell.Col++
	} else if nextRowOnEnd && m.SelectedCell.Row < len(m.Data)-1 {
		// Move to next row, first column
		m.SelectedCell.Row++
		m.SelectedCell.Col = 0
	}
}

func (m *Model) moveLeft(nextRowOnEnd bool) {
	if m.SelectedCell.Col > 0 {
		m.SelectedCell.Col--
	} else if nextRowOnEnd && m.SelectedCell.Row > 0 {
		// Move to previous row, last column
		m.SelectedCell.Row--
		maxCol := len(m.Header) - 1
		if len(m.Data[m.SelectedCell.Row]) > 0 {
			maxCol = min(maxCol, len(m.Data[m.SelectedCell.Row])-1)
		}
		m.SelectedCell.Col = maxCol
	}
}

func (m *Model) moveUp() {
	if m.SelectedCell.Row > 0 {
		m.SelectedCell.Row--
		// Ensure column is valid for the new row
		if m.SelectedCell.Col >= len(m.Data[m.SelectedCell.Row]) {
			m.SelectedCell.Col = len(m.Data[m.SelectedCell.Row]) - 1
		}
	}
}

func (m *Model) moveDown() {
	if m.SelectedCell.Row < len(m.Data)-1 {
		m.SelectedCell.Row++
		// Ensure column is valid for the new row
		if m.SelectedCell.Col >= len(m.Data[m.SelectedCell.Row]) {
			m.SelectedCell.Col = len(m.Data[m.SelectedCell.Row]) - 1
		}
	}
}

func (m *Model) View() string {
	header := TruncateStyledLines(m.headerView(), m.hScrollOffset, m.hScrollOffset+m.width)

	firstVisibleRow := m.viewport.YOffset / 2
	lastVisibleRow := (m.viewport.YOffset + m.viewport.Height + 1) / 2

	buffer := 2
	startRow := max(0, firstVisibleRow-buffer)
	endRow := min(len(m.Data), lastVisibleRow+buffer)

	visibleRowCount := endRow - startRow
	dataRows := make([]string, visibleRowCount+1)

	for i := startRow; i < endRow; i++ {
		dataRows[i-startRow] = m.rowView(m.Data[i], i)
	}
	dataRows[len(dataRows)-1] = m.bottomBorderView()

	dataContent := lipgloss.JoinVertical(lipgloss.Top, dataRows...)
	dataContent = TruncateStyledLines(dataContent, m.hScrollOffset, m.hScrollOffset+m.width)

	emptyLine := strings.Repeat(" ", m.width)
	linesAbove := startRow * 2
	if linesAbove > 0 {
		paddingAbove := strings.Repeat(emptyLine+"\n", linesAbove)
		dataContent = paddingAbove + dataContent
	}

	linesBelow := (len(m.Data) - endRow) * 2
	if linesBelow > 0 {
		paddingBelow := strings.Repeat(emptyLine+"\n", linesBelow)
		dataContent = dataContent + paddingBelow
	}

	m.viewport.SetContent(dataContent)
	viewportView := m.viewport.View()

	m.ensureSelectedCellVisible()
	return lipgloss.JoinVertical(lipgloss.Top, header, viewportView)
}

func (m *Model) headerView() string {
	if len(m.colWidths) == 0 {
		return ""
	}

	var topBorder = ""
	var headerData = ""
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	for i, col := range m.Header {
		width := m.colWidths[i]
		truncated := truncateString(col, width)

		if i == 0 {
			topBorder += borderStyle.Render("╭") + borderStyle.Render(strings.Repeat("─", width))
		} else {
			topBorder += borderStyle.Render("┬") + borderStyle.Render(strings.Repeat("─", width))
		}

		headerData += borderStyle.Render("│") + headerStyle.Render(truncated)

		if i == len(m.Header)-1 {
			topBorder += borderStyle.Render("╮")
			headerData += borderStyle.Render("│")
		}
	}
	return lipgloss.JoinVertical(lipgloss.Top, topBorder, headerData)
}

func (m *Model) rowView(row []any, rowNum int) string {
	if len(m.colWidths) == 0 {
		return ""
	}

	var topBorder = ""
	var rowData = ""
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	// Check if this is the first visible row and if there are more rows above
	isFirstVisibleRow := (rowNum * 2) == m.viewport.YOffset
	showMoreAbove := isFirstVisibleRow && m.hasMoreRowsAbove()

	for i, cell := range row {
		isSelected := m.Focus && m.SelectedCell.Row == rowNum && m.SelectedCell.Col == i

		if i >= len(m.colWidths) {
			break
		}

		width := m.colWidths[i]
		cellStr := fmt.Sprintf("%v", cell)
		truncated := truncateString(cellStr, width)

		// Use different border characters if this is first visible row with more above
		if i == 0 {
			if showMoreAbove {
				topBorder += borderStyle.Render("╞") + borderStyle.Render(strings.Repeat("═", width))
			} else {
				topBorder += borderStyle.Render("├") + borderStyle.Render(strings.Repeat("─", width))
			}
		} else {
			if showMoreAbove {
				topBorder += borderStyle.Render("╪") + borderStyle.Render(strings.Repeat("═", width))
			} else {
				topBorder += borderStyle.Render("┼") + borderStyle.Render(strings.Repeat("─", width))
			}
		}

		style := dataStyle
		if isSelected {
			style = selectedDataStyle
		}
		rowData += borderStyle.Render("│") + style.Render(truncated)

		if i == len(m.colWidths)-1 {
			if showMoreAbove {
				topBorder += borderStyle.Render("╡")
			} else {
				topBorder += borderStyle.Render("┤")
			}
			rowData += borderStyle.Render("│")
		}
	}
	return lipgloss.JoinVertical(lipgloss.Top, topBorder, rowData)
}

func (m *Model) bottomBorderView() string {
	if len(m.colWidths) == 0 {
		return ""
	}

	var bottomBorder = ""
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	showMoreBelow := m.hasMoreRowsBelow()

	for i, width := range m.colWidths {
		if i == 0 {
			if showMoreBelow {
				bottomBorder += borderStyle.Render("├") + borderStyle.Render(strings.Repeat("─", width))
			} else {
				bottomBorder += borderStyle.Render("╰") + borderStyle.Render(strings.Repeat("─", width))
			}
		} else {
			if showMoreBelow {
				bottomBorder += borderStyle.Render("┼") + borderStyle.Render(strings.Repeat("─", width))
			} else {
				bottomBorder += borderStyle.Render("┴") + borderStyle.Render(strings.Repeat("─", width))
			}
		}

		if i == len(m.colWidths)-1 {
			if showMoreBelow {
				bottomBorder += borderStyle.Render("┤")
			} else {
				bottomBorder += borderStyle.Render("╯")
			}
		}
	}

	return bottomBorder
}

func (m *Model) SelectedCellValue() string {
	if m.SelectedCell.Row < len(m.Data) && m.SelectedCell.Col < len(m.Data[m.SelectedCell.Row]) {
		return fmt.Sprintf("%v", m.Data[m.SelectedCell.Row][m.SelectedCell.Col])
	}
	return ""
}

func (m *Model) SelectedRowValues() []string {
	if m.SelectedCell.Row >= len(m.Data) {
		return nil
	}
	row := m.Data[m.SelectedCell.Row]
	vals := make([]string, len(row))
	for i, v := range row {
		vals[i] = fmt.Sprintf("%v", v)
	}
	return vals
}

func (m *Model) SetWidth(width int) {
	m.width = width
	m.colWidths = m.CalculateColumnWidths(width)
	m.viewport.Width = width
}

func (m *Model) SetHeight(height int) {
	if height%2 == 0 {
		height = height - 1 // Ensure height is odd to accommodate header and data rows
	}
	m.height = height

	// Reserve space for header (2 lines: top border + header data)
	viewportHeight := max(height-2, 1)
	m.viewport.Height = viewportHeight
	m.ensureSelectedCellVisible()
}

func NewModel(header []string, data [][]any, width, height int) *Model {
	vpModel := viewport.New(width, 0)
	m := &Model{
		Header:        header,
		Data:          data,
		width:         width,
		viewport:      &vpModel,
		hScrollOffset: 0,
	}
	m.SetHeight(height)
	m.colWidths = m.CalculateColumnWidths(width)
	return m
}

func (m *Model) CalculateColumnWidths(containerWidth int) []int {
	const colMin = 15
	numCols := len(m.Header)

	if numCols == 0 {
		return []int{}
	}

	actualWidths := make([]int, numCols)
	for i, header := range m.Header {
		actualWidths[i] = len(header)
	}

	for _, row := range m.Data {
		for i, cell := range row {
			if i < numCols {
				cellStr := strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v", cell), "\n", " "), "\r", "")
				if len(cellStr) > actualWidths[i] {
					actualWidths[i] = len(cellStr)
				}
			}
		}
	}

	type colInfo struct {
		index       int
		actualWidth int
	}
	cols := make([]colInfo, numCols)
	for i := range cols {
		cols[i] = colInfo{index: i, actualWidth: actualWidths[i]}
	}
	sort.Slice(cols, func(i, j int) bool {
		return cols[i].actualWidth < cols[j].actualWidth
	})

	finalWidths := make([]int, numCols)
	remainingWidth := containerWidth
	remainingCols := numCols

	for _, col := range cols {
		allottedWidth := remainingWidth / remainingCols
		width := min(col.actualWidth, max(allottedWidth, colMin))

		finalWidths[col.index] = width
		remainingWidth -= width
		remainingCols--
	}

	return finalWidths
}

func (m *Model) ensureSelectedCellVisible() {
	// Handle vertical scrolling

	selectedRowY := m.SelectedCell.Row*2 + 1

	if selectedRowY < m.viewport.YOffset {
		m.viewport.SetYOffset(selectedRowY - 1)
	}

	if selectedRowY >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(selectedRowY - m.viewport.Height + 2)
	}

	// Handle horizontal scrolling

	fullWidth := m.actualWidth()
	hStart, hEnd := m.horizontalRangeOfCell(m.SelectedCell)
	if hEnd > m.hScrollOffset+m.width {
		m.hScrollOffset = min(hEnd+1, fullWidth) - m.width
	} else if hStart < m.hScrollOffset {
		m.hScrollOffset = max(hStart-1, 0)
	}
}

func (m *Model) actualWidth() int {
	if len(m.colWidths) == 0 {
		return 0
	}

	totalWidth := 0

	// Add left border
	totalWidth += 1

	// Add all column widths
	for _, width := range m.colWidths {
		totalWidth += width
	}

	// Add column separators (one less than number of columns)
	totalWidth += len(m.colWidths) - 1

	// Add right border
	totalWidth += 1

	return totalWidth
}

func (m *Model) horizontalRangeOfCell(cell CellRef) (int, int) {
	if cell.Col < 0 || cell.Col >= len(m.colWidths) {
		return 0, 0
	}

	// Calculate starting position
	start := 0
	// Account for left border
	start += 1

	// Add widths of all columns before this one
	for i := 0; i < cell.Col; i++ {
		start += m.colWidths[i] + 1 // +1 for the column separator
	}

	// End position is start + column width
	end := start + m.colWidths[cell.Col] + 1

	return start, end
}

func (m *Model) hasMoreRowsAbove() bool {
	return m.viewport.YOffset > 0
}

func (m *Model) hasMoreRowsBelow() bool {
	// Total content height: each data row is 2 lines (border + content) + 1 line for bottom border
	totalContentHeight := len(m.Data)*2 + 1
	return m.viewport.YOffset+m.viewport.Height < totalContentHeight
}
