package main

import (
	"bytes"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/quakan/tabletui/modal"
	"github.com/quakan/tabletui/table"
)

var (
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#89b5f9"))
	hintStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
)

type model struct {
	tbl       *table.Model
	modal     modal.Model
	delimiter rune
	width     int
	height    int
	message   string
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.tbl.SetWidth(msg.Width)
		m.tbl.SetHeight(msg.Height - 1)
		m.modal.Width = msg.Width
		m.modal.Height = msg.Height
		return m, nil
	case modal.CloseMsg:
		m.modal.Show = false
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.modal.Show {
			var cmd tea.Cmd
			m.modal, cmd = m.modal.Update(msg)
			return m, cmd
		}

		m.message = ""
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "y":
			m.message = m.yank(m.tbl.SelectedCellValue(), "cell")
			return m, nil
		case "Y":
			m.message = m.yank(m.encodeRow(m.tbl.SelectedRowValues()), "row")
			return m, nil
		case "o", "enter":
			m.openCellDetail()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)
	return m, cmd
}

func (m *model) openCellDetail() {
	col := m.tbl.SelectedCell.Col
	if col < 0 || col >= len(m.tbl.Header) {
		return
	}
	header := m.tbl.Header[col]
	m.modal = modal.Model{
		Content:     modal.NewCellDetailModalContent(header, m.tbl.SelectedCellValue()),
		Show:        true,
		BorderColor: "#EFB68F",
		Title:       header,
		Width:       m.width,
		Height:      m.height,
		X:           -1,
		Y:           -1,
	}
}

func (m model) yank(val, what string) string {
	if val == "" {
		return "Nothing to yank"
	}
	if err := clipboard.WriteAll(val); err != nil {
		return "Yank failed: " + err.Error()
	}
	return fmt.Sprintf("Yanked %s to clipboard", what)
}

func (m model) encodeRow(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = m.delimiter
	w.Write(vals)
	w.Flush()
	return strings.TrimRight(buf.String(), "\n")
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	view := lipgloss.JoinVertical(lipgloss.Left, m.tbl.View(), m.statusView())
	if m.modal.Show {
		m.modal.BackgroundView = view
		return m.modal.View()
	}
	return view
}

func (m model) statusView() string {
	sel := m.tbl.SelectedCell
	left := fmt.Sprintf(" row %d/%d  col %d/%d", sel.Row+1, len(m.tbl.Data), sel.Col+1, len(m.tbl.Header))
	if sel.Col >= 0 && sel.Col < len(m.tbl.Header) {
		left += "  " + m.tbl.Header[sel.Col]
	}

	right := m.message
	if right == "" {
		right = "hjkl move · o open · y/Y yank cell/row · q quit"
	}
	right += " "

	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	line := statusStyle.Render(left) + strings.Repeat(" ", gap) + hintStyle.Render(right)
	return table.TruncateStyledLines(line, 0, m.width)
}

func readRecords(r io.Reader, delimiter rune) ([][]string, error) {
	cr := csv.NewReader(r)
	cr.Comma = delimiter
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	return cr.ReadAll()
}

// splitTable separates the header from the data rows and pads every row to
// the same width so the table's navigation sees a rectangular grid.
func splitTable(records [][]string, hasHeader bool) ([]string, [][]string) {
	numCols := 0
	for _, rec := range records {
		numCols = max(numCols, len(rec))
	}

	var header []string
	if hasHeader {
		header, records = records[0], records[1:]
	}
	for len(header) < numCols {
		if hasHeader {
			header = append(header, "")
		} else {
			header = append(header, fmt.Sprintf("column%d", len(header)+1))
		}
	}

	rows := make([][]string, len(records))
	for i, rec := range records {
		row := make([]string, numCols)
		copy(row, rec)
		rows[i] = row
	}
	return header, rows
}

func parseDelimiter(s, path string) (rune, error) {
	switch s {
	case "":
		if strings.EqualFold(filepath.Ext(path), ".tsv") {
			return '\t', nil
		}
		return ',', nil
	case `\t`, "tab":
		return '\t', nil
	}
	r := []rune(s)
	if len(r) != 1 {
		return 0, fmt.Errorf("delimiter must be a single character, got %q", s)
	}
	return r[0], nil
}

func run() error {
	delimFlag := flag.String("d", "", `field delimiter (default "," or tab for .tsv files; use "\t" for tab)`)
	noHeader := flag.Bool("no-header", false, "treat the first row as data instead of column names")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: tabletui [flags] [file]\n\nReads CSV from file, or from stdin when no file (or \"-\") is given.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	path := flag.Arg(0)

	var input io.Reader
	if path == "" || path == "-" {
		if stat, err := os.Stdin.Stat(); err == nil && stat.Mode()&os.ModeCharDevice != 0 {
			flag.Usage()
			os.Exit(2)
		}
		input = os.Stdin
		path = ""
	} else {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		input = f
	}

	delimiter, err := parseDelimiter(*delimFlag, path)
	if err != nil {
		return err
	}

	records, err := readRecords(input, delimiter)
	if err != nil {
		return fmt.Errorf("parsing input: %w", err)
	}
	if len(records) == 0 {
		return errors.New("no data")
	}

	header, rows := splitTable(records, !*noHeader)
	tbl := table.NewModel(header, table.ConvertStringRows(rows), 0, 0)
	tbl.Focus = true

	p := tea.NewProgram(
		model{tbl: tbl, delimiter: delimiter},
		tea.WithAltScreen(),
		tea.WithInputTTY(),
	)
	_, err = p.Run()
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "tabletui:", err)
		os.Exit(1)
	}
}
