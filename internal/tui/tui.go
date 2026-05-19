package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/korECM/openapi-extract/internal/catalog"
	"github.com/korECM/openapi-extract/internal/extractor"
	"github.com/korECM/openapi-extract/internal/output"
)

type mode int

const (
	modeList mode = iota
	modeSearch
	modeSave
)

type model struct {
	raw       map[string]any
	ops       []catalog.Operation
	filtered  []catalog.Operation
	selected  map[string]catalog.Operation
	cursor    int
	query     string
	savePath  string
	mode      mode
	message   string
	quitting  bool
	termWidth int
}

func Run(raw map[string]any, ops []catalog.Operation) error {
	m := model{
		raw:      raw,
		ops:      ops,
		filtered: ops,
		selected: map[string]catalog.Operation{},
		message:  "space select · / search · c copy · s save · y stdout · q quit",
	}
	_, err := tea.NewProgram(m).Run()
	return err
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
	case tea.KeyMsg:
		switch m.mode {
		case modeSearch:
			return m.updateSearch(msg)
		case modeSave:
			return m.updateSave(msg)
		default:
			return m.updateList(msg)
		}
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case "/":
		m.mode = modeSearch
		m.message = "type to search · enter apply · esc cancel"
	case " ":
		if len(m.filtered) == 0 {
			break
		}
		op := m.filtered[m.cursor]
		if _, ok := m.selected[op.ID]; ok {
			delete(m.selected, op.ID)
		} else {
			m.selected[op.ID] = op
		}
	case "c":
		m = m.writeSelection("copy", "")
	case "s":
		if len(m.selected) == 0 {
			m.message = "select at least one operation before saving"
			break
		}
		m.mode = modeSave
		m.savePath = "mini.openapi.yaml"
		m.message = "enter output path · enter save · esc cancel"
	case "y":
		m = m.writeSelection("stdout", "")
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.message = "↑/↓ or j/k move · / search · space select · c copy · s save · y stdout · q quit"
	}
	return m, nil
}

func (m model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.applyFilter()
		m.mode = modeList
		m.message = fmt.Sprintf("filtered %d of %d operations", len(m.filtered), len(m.ops))
	case "esc":
		m.mode = modeList
		m.message = "search canceled"
	case "backspace":
		if len(m.query) > 0 {
			m.query = m.query[:len(m.query)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.query += msg.String()
		}
	}
	return m, nil
}

func (m model) updateSave(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m = m.writeSelection("file", m.savePath)
		m.mode = modeList
	case "esc":
		m.mode = modeList
		m.message = "save canceled"
	case "backspace":
		if len(m.savePath) > 0 {
			m.savePath = m.savePath[:len(m.savePath)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.savePath += msg.String()
		}
	}
	return m, nil
}

func (m *model) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.query))
	if query == "" {
		m.filtered = m.ops
		m.cursor = 0
		return
	}
	m.filtered = m.filtered[:0]
	for _, op := range m.ops {
		haystack := strings.ToLower(strings.Join([]string{
			op.ID,
			op.Method,
			op.Path,
			op.OperationID,
			op.Summary,
			strings.Join(op.Tags, " "),
		}, " "))
		if strings.Contains(haystack, query) {
			m.filtered = append(m.filtered, op)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m model) writeSelection(target, path string) model {
	selected := m.selectedOps()
	if len(selected) == 0 {
		m.message = "select at least one operation first"
		return m
	}
	mini, err := extractor.Extract(m.raw, selected, extractor.Options{})
	if err != nil {
		m.message = err.Error()
		return m
	}
	data, err := output.Marshal(mini, "yaml")
	if err != nil {
		m.message = err.Error()
		return m
	}
	switch target {
	case "copy":
		if err := output.Copy(data); err != nil {
			m.message = err.Error()
		} else {
			m.message = fmt.Sprintf("copied %d operation(s) to clipboard", len(selected))
		}
	case "file":
		if err := output.WriteFile(path, data); err != nil {
			m.message = err.Error()
		} else {
			m.message = fmt.Sprintf("saved %d operation(s) to %s", len(selected), path)
		}
	case "stdout":
		fmt.Print(string(data))
	}
	return m
}

func (m model) selectedOps() []catalog.Operation {
	ops := make([]catalog.Operation, 0, len(m.selected))
	for _, op := range m.ops {
		if selected, ok := m.selected[op.ID]; ok {
			ops = append(ops, selected)
		}
	}
	return ops
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")).Render("openapi-extract")
	fmt.Fprintf(&b, "%s  %d operations · selected %d\n\n", title, len(m.ops), len(m.selected))

	switch m.mode {
	case modeSearch:
		fmt.Fprintf(&b, "/ %s\n\n", m.query)
	case modeSave:
		fmt.Fprintf(&b, "save as: %s\n\n", m.savePath)
	default:
		if m.query != "" {
			fmt.Fprintf(&b, "filter: %s\n\n", m.query)
		}
	}

	start := max(0, m.cursor-8)
	end := min(len(m.filtered), start+16)
	for i := start; i < end; i++ {
		op := m.filtered[i]
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		selected := false
		check := "○"
		if _, ok := m.selected[op.ID]; ok {
			selected = true
			check = "x"
		}
		method := methodStyle(op.Method).Render(fmt.Sprintf("%-6s", op.Method))
		pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
		metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
		cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)
		if selected {
			pathStyle = pathStyle.Copy().Bold(true)
			metaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("121"))
			check = selectedStyle.Render("●")
		} else {
			check = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(check)
		}
		line := fmt.Sprintf("%s %s %s %s", cursor, check, method, pathStyle.Render(op.Path))
		if op.Summary != "" {
			line += metaStyle.Render(" - " + op.Summary)
		}
		if i == m.cursor {
			line = cursorStyle.Render(line)
		}
		fmt.Fprintln(&b, line)
		if len(op.Tags) > 0 || op.OperationID != "" {
			meta := fmt.Sprintf("      %s %s", strings.Join(op.Tags, ","), op.OperationID)
			fmt.Fprintln(&b, metaStyle.Render(meta))
		}
	}

	fmt.Fprintf(&b, "\n%s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(m.message))
	return b.String()
}

func methodStyle(method string) lipgloss.Style {
	switch method {
	case "GET":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	case "POST":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	case "PUT", "PATCH":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	case "DELETE":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	default:
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("147"))
	}
}
