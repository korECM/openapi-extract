package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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
	modeMethodFilter
	modeTagFilter
	modeHelp
)

const (
	hintIdle   = "space sel · / search · m method · t tag · S selected · e/i opts · F fmt · f full · c copy · s save · y stdout · ? help · q quit"
	hintSearch = "type to filter live · enter apply · esc clear"
	hintSave   = "type path · enter save · esc cancel"
	hintMethod = "g/p/u/a/d/o/h/t toggle · c clear · esc done"
	hintTag    = "j/k move · space toggle · c clear · esc done"
	hintHelp   = "any key returns"
)

const (
	fmtYAML = "yaml"
	fmtJSON = "json"
)

var methodOrder = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD", "TRACE"}

var methodKey = map[string]string{
	"g": "GET",
	"p": "POST",
	"u": "PUT",
	"a": "PATCH",
	"d": "DELETE",
	"o": "OPTIONS",
	"h": "HEAD",
	"t": "TRACE",
}

type model struct {
	raw      map[string]any
	ops      []catalog.Operation
	tags     []string
	methods  map[string]bool
	filtered []catalog.Operation
	selected map[string]catalog.Operation

	cursor int
	offset int

	searchInput textinput.Model
	saveInput   textinput.Model

	methodFilter map[string]bool
	tagFilter    map[string]bool
	tagCursor    int

	mode    mode
	message string

	width  int
	height int

	showSelectedOnly bool
	fullscreen       bool
	outputFormat     string
	extractOpts      extractor.Options

	detailVP         viewport.Model
	detailFocus      bool
	detailLastCursor int
	detailReady      bool

	stdoutOut []byte
	quitting  bool
}

// Run launches the interactive TUI. Any data the user asked to print to
// stdout is returned as the first value so the caller can write it after the
// TUI has restored the terminal (avoids mixing TUI frames with payload).
func Run(raw map[string]any, ops []catalog.Operation) ([]byte, error) {
	search := textinput.New()
	search.Placeholder = "filter by path, method, tag, summary, operationId"
	search.Prompt = "/ "
	search.CharLimit = 256

	save := textinput.New()
	save.Placeholder = "mini.openapi.yaml"
	save.Prompt = "save as: "
	save.CharLimit = 1024
	save.SetValue("mini.openapi.yaml")

	m := model{
		raw:          raw,
		ops:          ops,
		tags:         uniqueTags(ops),
		methods:      methodSet(ops),
		filtered:     ops,
		selected:     map[string]catalog.Operation{},
		searchInput:  search,
		saveInput:    save,
		methodFilter: map[string]bool{},
		tagFilter:    map[string]bool{},
		width:        80,
		height:       24,
		outputFormat: fmtYAML,
		message:      hintIdle,
	}
	final, err := tea.NewProgram(m, tea.WithMouseCellMotion()).Run()
	if err != nil {
		return nil, err
	}
	if fm, ok := final.(model); ok {
		return fm.stdoutOut, nil
	}
	return nil, nil
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureCursorVisible()
		return m, nil
	case tea.MouseMsg:
		if m.mode != modeList && m.mode != modeTagFilter {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.mode == modeTagFilter {
				if m.tagCursor > 0 {
					m.tagCursor--
				}
			} else if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
			}
		case tea.MouseButtonWheelDown:
			if m.mode == modeTagFilter {
				if m.tagCursor < len(m.tags)-1 {
					m.tagCursor++
				}
			} else if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}
		}
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case modeSearch:
			return m.updateSearch(msg)
		case modeSave:
			return m.updateSave(msg)
		case modeMethodFilter:
			return m.updateMethod(msg)
		case modeTagFilter:
			return m.updateTag(msg)
		case modeHelp:
			m.mode = modeList
			m.message = hintIdle
			return m, nil
		default:
			if m.detailFocus {
				return m.updateDetail(msg)
			}
			return m.updateList(msg)
		}
	}
	return m, nil
}

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab", "esc":
		m.detailFocus = false
		m.message = "list focused"
		return m, nil
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
		m.message = hintHelp
		return m, nil
	case "f":
		if m.width >= 100 {
			m.fullscreen = !m.fullscreen
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.detailVP, cmd = m.detailVP.Update(msg)
	return m, cmd
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
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
	case "pgup", "ctrl+u":
		m.cursor -= m.listHeight()
		if m.cursor < 0 {
			m.cursor = 0
		}
	case "pgdown", "ctrl+d":
		m.cursor += m.listHeight()
		if m.cursor >= len(m.filtered) {
			m.cursor = max(0, len(m.filtered)-1)
		}
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = max(0, len(m.filtered)-1)
	case "/":
		m.mode = modeSearch
		m.searchInput.Focus()
		m.message = hintSearch
		return m, textinput.Blink
	case " ":
		if len(m.filtered) > 0 {
			op := m.filtered[m.cursor]
			if _, ok := m.selected[op.ID]; ok {
				delete(m.selected, op.ID)
			} else {
				m.selected[op.ID] = op
			}
		}
	case "a":
		for _, op := range m.filtered {
			m.selected[op.ID] = op
		}
		m.message = fmt.Sprintf("selected all %d visible", len(m.filtered))
	case "A":
		m.selected = map[string]catalog.Operation{}
		m.message = "selection cleared"
	case "m":
		m.mode = modeMethodFilter
		m.message = hintMethod
	case "t":
		m.mode = modeTagFilter
		if m.tagCursor >= len(m.tags) {
			m.tagCursor = 0
		}
		m.message = hintTag
	case "c":
		m = m.writeSelection("copy", "")
	case "s":
		if len(m.selected) == 0 {
			m.message = "select at least one operation before saving"
			break
		}
		m.mode = modeSave
		m.saveInput.Focus()
		m.message = hintSave
		return m, textinput.Blink
	case "y":
		if len(m.selected) == 0 {
			m.message = "select at least one operation before printing"
			break
		}
		m = m.writeSelection("stdout", "")
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
		m.message = hintHelp
	case "S":
		m.showSelectedOnly = !m.showSelectedOnly
		m.applyFilter()
		if m.showSelectedOnly {
			m.message = fmt.Sprintf("showing only selected (%d)", len(m.selected))
		} else {
			m.message = "showing all operations"
		}
	case "e":
		m.extractOpts.DropExamples = !m.extractOpts.DropExamples
		m.message = "drop-examples: " + onOff(m.extractOpts.DropExamples)
	case "i":
		m.extractOpts.StripInfoDescription = !m.extractOpts.StripInfoDescription
		m.message = "strip-info-description: " + onOff(m.extractOpts.StripInfoDescription)
	case "F":
		if m.outputFormat == fmtYAML {
			m.outputFormat = fmtJSON
		} else {
			m.outputFormat = fmtYAML
		}
		m.message = "output format: " + m.outputFormat
	case "f":
		if m.width >= 100 {
			m.fullscreen = !m.fullscreen
			if m.fullscreen {
				m.message = "detail fullscreen on"
			} else {
				m.message = "detail fullscreen off"
			}
		} else {
			m.message = "fullscreen needs width ≥ 100"
		}
	case "tab", "shift+tab":
		if m.width >= 100 || m.fullscreen {
			m.detailFocus = true
			m.message = "detail focused — j/k scroll, tab back"
		}
	}
	m.ensureCursorVisible()
	return m, nil
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func (m model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.searchInput.SetValue("")
		m.searchInput.Blur()
		m.mode = modeList
		m.applyFilter()
		m.message = "search cleared"
		return m, nil
	case tea.KeyEnter:
		m.searchInput.Blur()
		m.mode = modeList
		m.message = m.filterStatus()
		return m, nil
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m model) updateSave(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.saveInput.Blur()
		m.mode = modeList
		m.message = "save canceled"
		return m, nil
	case tea.KeyEnter:
		m.saveInput.Blur()
		m.mode = modeList
		m = m.writeSelection("file", strings.TrimSpace(m.saveInput.Value()))
		return m, nil
	}
	var cmd tea.Cmd
	m.saveInput, cmd = m.saveInput.Update(msg)
	return m, cmd
}

func (m model) updateMethod(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "m":
		m.mode = modeList
		m.message = m.filterStatus()
		return m, nil
	case "c":
		m.methodFilter = map[string]bool{}
		m.applyFilter()
		m.ensureCursorVisible()
		return m, nil
	}
	if method, ok := methodKey[strings.ToLower(msg.String())]; ok {
		if !m.methods[method] {
			return m, nil
		}
		if m.methodFilter[method] {
			delete(m.methodFilter, method)
		} else {
			m.methodFilter[method] = true
		}
		m.applyFilter()
		m.ensureCursorVisible()
	}
	return m, nil
}

func (m model) updateTag(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "t":
		m.mode = modeList
		m.message = m.filterStatus()
		return m, nil
	case "j", "down":
		if m.tagCursor < len(m.tags)-1 {
			m.tagCursor++
		}
	case "k", "up":
		if m.tagCursor > 0 {
			m.tagCursor--
		}
	case "g", "home":
		m.tagCursor = 0
	case "G", "end":
		m.tagCursor = max(0, len(m.tags)-1)
	case " ":
		if len(m.tags) > 0 {
			tag := m.tags[m.tagCursor]
			if m.tagFilter[tag] {
				delete(m.tagFilter, tag)
			} else {
				m.tagFilter[tag] = true
			}
			m.applyFilter()
			m.ensureCursorVisible()
		}
	case "c":
		m.tagFilter = map[string]bool{}
		m.applyFilter()
		m.ensureCursorVisible()
	}
	return m, nil
}

func (m *model) applyFilter() {
	out := m.ops
	if m.showSelectedOnly {
		sel := make([]catalog.Operation, 0, len(m.selected))
		for _, op := range m.ops {
			if _, ok := m.selected[op.ID]; ok {
				sel = append(sel, op)
			}
		}
		out = sel
	}
	if len(m.methodFilter) > 0 {
		out = catalog.FilterByMethods(out, sortedKeys(m.methodFilter))
	}
	if len(m.tagFilter) > 0 {
		out = catalog.FilterByTags(out, sortedKeys(m.tagFilter))
	}
	query := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
	if query != "" {
		next := make([]catalog.Operation, 0, len(out))
		for _, op := range out {
			hay := strings.ToLower(strings.Join([]string{
				op.ID, op.Method, op.Path, op.OperationID, op.Summary, strings.Join(op.Tags, " "),
			}, " "))
			if strings.Contains(hay, query) {
				next = append(next, op)
			}
		}
		out = next
	}
	m.filtered = out
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m *model) ensureCursorVisible() {
	h := m.listHeight()
	if h <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	maxOffset := max(0, len(m.filtered)-h)
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m model) writeSelection(target, path string) model {
	selected := m.selectedOps()
	if len(selected) == 0 {
		m.message = "select at least one operation first"
		return m
	}
	mini, err := extractor.Extract(m.raw, selected, m.extractOpts)
	if err != nil {
		m.message = err.Error()
		return m
	}
	data, err := output.Marshal(mini, m.outputFormat)
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
		if path == "" {
			m.message = "save path is empty"
			return m
		}
		if err := output.WriteFile(path, data); err != nil {
			m.message = err.Error()
		} else {
			m.message = fmt.Sprintf("saved %d operation(s) to %s", len(selected), path)
		}
	case "stdout":
		m.stdoutOut = data
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

func (m model) filterStatus() string {
	parts := []string{fmt.Sprintf("filtered %d of %d", len(m.filtered), len(m.ops))}
	if len(m.methodFilter) > 0 {
		parts = append(parts, "methods: "+strings.Join(sortedKeys(m.methodFilter), ","))
	}
	if len(m.tagFilter) > 0 {
		parts = append(parts, "tags: "+strings.Join(sortedKeys(m.tagFilter), ","))
	}
	if v := strings.TrimSpace(m.searchInput.Value()); v != "" {
		parts = append(parts, "search: "+v)
	}
	return strings.Join(parts, " · ")
}

// ---------- View ----------

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.mode == modeHelp {
		return m.renderHelp()
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	showDetail := m.width >= 100
	if m.fullscreen && showDetail {
		body := m.renderDetail(m.width, bodyHeight)
		return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	}

	listWidth := m.width
	if showDetail {
		listWidth = m.width * 5 / 9
	}

	var left string
	if m.mode == modeTagFilter {
		left = m.renderTagPicker(listWidth, bodyHeight)
	} else {
		left = m.renderList(listWidth, bodyHeight)
	}

	body := left
	if showDetail {
		right := m.renderDetail(m.width-listWidth-3, bodyHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, verticalSep(bodyHeight, m.detailFocus), right)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func verticalSep(height int, focused bool) string {
	char := "│"
	color := lipgloss.Color("238")
	if focused {
		char = "┃"
		color = lipgloss.Color("214")
	}
	bars := make([]string, height)
	for i := range bars {
		bars[i] = char
	}
	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Join(bars, "\n"))
	return " " + bar + " "
}

func (m model) renderHeader() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")).Render("openapi-extract")
	stats := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(
		fmt.Sprintf("%d ops · %d selected · showing %d", len(m.ops), len(m.selected), len(m.filtered)),
	)
	rows := []string{title + "  " + stats}

	chips := m.activeChips()
	if len(chips) > 0 {
		rows = append(rows, strings.Join(chips, " "))
	}

	switch m.mode {
	case modeSearch:
		rows = append(rows, m.searchInput.View())
	case modeSave:
		rows = append(rows, m.saveInput.View())
	case modeMethodFilter:
		rows = append(rows, m.renderMethodPicker())
	}
	rows = append(rows, "")
	return strings.Join(rows, "\n")
}

func (m model) activeChips() []string {
	chip := lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("236")).Foreground(lipgloss.Color("228"))
	var chips []string
	if len(m.methodFilter) > 0 {
		chips = append(chips, chip.Render("methods: "+strings.Join(sortedKeys(m.methodFilter), ",")))
	}
	if len(m.tagFilter) > 0 {
		chips = append(chips, chip.Render("tags: "+strings.Join(sortedKeys(m.tagFilter), ",")))
	}
	if v := strings.TrimSpace(m.searchInput.Value()); v != "" && m.mode != modeSearch {
		chips = append(chips, chip.Render("search: "+v))
	}
	return chips
}

func (m model) renderMethodPicker() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	var parts []string
	for _, mn := range methodOrder {
		if !m.methods[mn] {
			continue
		}
		key := keyForMethod(mn)
		on := m.methodFilter[mn]
		mark := "○"
		if on {
			mark = "●"
		}
		text := fmt.Sprintf("[%s] %s %s", key, mark, mn)
		if on {
			text = methodStyle(mn).Render(text)
		} else {
			text = dim.Render(text)
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "  ")
}

func (m model) renderList(width, height int) string {
	if len(m.filtered) == 0 {
		empty := lipgloss.NewStyle().Width(width).Height(height).Align(lipgloss.Center, lipgloss.Center).Foreground(lipgloss.Color("244"))
		return empty.Render("no operations match filter")
	}

	end := m.offset + height
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	var lines []string
	for i := m.offset; i < end; i++ {
		op := m.filtered[i]
		_, selected := m.selected[op.ID]
		lines = append(lines, renderRow(op, i == m.cursor, selected, width))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderRow(op catalog.Operation, isCursor, isSelected bool, width int) string {
	check := "○"
	checkColor := lipgloss.Color("240")
	if isSelected {
		check = "●"
		checkColor = lipgloss.Color("42")
	}
	checkStyle := lipgloss.NewStyle().Foreground(checkColor).Bold(isSelected)

	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	if isSelected {
		pathStyle = pathStyle.Bold(true)
	}
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	if isSelected {
		metaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("121"))
	}

	cursorMark := "  "
	if isCursor {
		cursorMark = "> "
	}

	method := methodStyle(op.Method).Render(fmt.Sprintf("%-7s", op.Method))
	line := fmt.Sprintf("%s%s %s %s", cursorMark, checkStyle.Render(check), method, pathStyle.Render(op.Path))
	if op.Summary != "" {
		line += metaStyle.Render("  " + op.Summary)
	}
	if op.Deprecated {
		line += lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(" [deprecated]")
	}

	if isCursor {
		return lipgloss.NewStyle().Width(width).Background(lipgloss.Color("236")).Render(line)
	}
	return lipgloss.NewStyle().Width(width).Render(line)
}

func (m *model) renderDetail(width, height int) string {
	if len(m.filtered) == 0 || width < 20 {
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}
	op := m.filtered[m.cursor]
	content := m.detailContent(op, width)

	if !m.detailReady {
		m.detailVP = viewport.New(width, height)
		m.detailReady = true
	}
	if m.detailVP.Width != width || m.detailVP.Height != height {
		m.detailVP.Width = width
		m.detailVP.Height = height
	}
	if m.detailLastCursor != m.cursor {
		m.detailVP.GotoTop()
		m.detailLastCursor = m.cursor
	}
	m.detailVP.SetContent(content)

	return m.detailVP.View()
}

func (m model) detailContent(op catalog.Operation, width int) string {
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	sepWidth := width
	if sepWidth > 80 {
		sepWidth = 80
	}
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(strings.Repeat("─", sepWidth))
	section := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))

	var lines []string
	lines = append(lines, methodStyle(op.Method).Render(op.Method)+"  "+pathStyle.Render(op.Path), sep)

	if op.Deprecated {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("⚠ DEPRECATED"))
	}
	if op.Summary != "" {
		lines = append(lines, fieldLine("Summary", op.Summary))
	}
	if op.OperationID != "" {
		lines = append(lines, fieldLine("OperationID", op.OperationID))
	}
	if len(op.Tags) > 0 {
		lines = append(lines, fieldLine("Tags", strings.Join(op.Tags, ", ")))
	}
	if len(op.Security) > 0 {
		lines = append(lines, fieldLine("Security", strings.Join(op.Security, ", ")))
	}
	if op.Kind == catalog.KindWebhook {
		lines = append(lines, fieldLine("Kind", "webhook"))
	}

	details := buildDetails(m.raw, op)

	if len(details.parameters) > 0 {
		lines = append(lines, "", section.Render("Parameters"))
		for _, p := range details.parameters {
			req := " "
			if p.required {
				req = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("*")
			}
			inStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
			nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Bold(true)
			lines = append(lines, fmt.Sprintf("  %s %s  %s  %s",
				req,
				nameStyle.Render(padr(p.name, 24)),
				inStyle.Render(padr(p.in, 7)),
				p.schema,
			))
			for _, sub := range p.expanded {
				lines = append(lines, "        "+sub)
			}
		}
	}

	if len(details.requestBody) > 0 {
		suffix := ""
		if op.RequestBodyRequired {
			suffix = " (required)"
		}
		lines = append(lines, "", section.Render("Request body"+suffix))
		for _, b := range details.requestBody {
			ct := lipgloss.NewStyle().Foreground(lipgloss.Color("147")).Render(padr(b.contentType, 26))
			lines = append(lines, "  "+ct+"  "+b.schema)
			for _, sub := range b.expanded {
				lines = append(lines, "      "+sub)
			}
		}
	}

	if len(details.responses) > 0 {
		lines = append(lines, "", section.Render("Responses"))
		for _, r := range details.responses {
			head := codeStyle(r.code).Render(padr(r.code, 8))
			if r.description != "" {
				head += " " + lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(r.description)
			}
			lines = append(lines, "  "+head)
			for _, b := range r.bodies {
				ct := lipgloss.NewStyle().Foreground(lipgloss.Color("147")).Render(padr(b.contentType, 24))
				lines = append(lines, "      "+ct+"  "+b.schema)
				for _, sub := range b.expanded {
					lines = append(lines, "          "+sub)
				}
			}
		}
	}

	if op.Description != "" {
		lines = append(lines, "", section.Render("Description"), wrap(op.Description, width))
	}

	return strings.Join(lines, "\n")
}

type paramInfo struct {
	name     string
	in       string
	required bool
	schema   string
	expanded []string
}

type bodyInfo struct {
	contentType string
	schema      string
	expanded    []string
}

type responseInfo struct {
	code        string
	description string
	bodies      []bodyInfo
}

type opDetails struct {
	parameters  []paramInfo
	requestBody []bodyInfo
	responses   []responseInfo
}

func buildDetails(raw map[string]any, op catalog.Operation) opDetails {
	var out opDetails
	container := "paths"
	if op.Kind == catalog.KindWebhook {
		container = "webhooks"
	}
	items, _ := raw[container].(map[string]any)
	if items == nil {
		return out
	}
	item, _ := items[op.Path].(map[string]any)
	if item == nil {
		return out
	}

	out.parameters = append(out.parameters, paramsFrom(raw, item["parameters"])...)

	opNode, _ := item[strings.ToLower(op.Method)].(map[string]any)
	if opNode == nil {
		return out
	}
	out.parameters = append(out.parameters, paramsFrom(raw, opNode["parameters"])...)

	if rb, ok := opNode["requestBody"].(map[string]any); ok {
		if ref, ok := rb["$ref"].(string); ok {
			if resolved := resolveRef(raw, ref); resolved != nil {
				rb = resolved
			}
		}
		out.requestBody = bodiesFromContent(raw, rb["content"])
	}

	if resps, ok := opNode["responses"].(map[string]any); ok {
		for _, code := range op.ResponseCodes {
			rNode, _ := resps[code].(map[string]any)
			if rNode == nil {
				continue
			}
			if ref, ok := rNode["$ref"].(string); ok {
				if resolved := resolveRef(raw, ref); resolved != nil {
					rNode = resolved
				}
			}
			desc, _ := rNode["description"].(string)
			out.responses = append(out.responses, responseInfo{
				code:        code,
				description: desc,
				bodies:      bodiesFromContent(raw, rNode["content"]),
			})
		}
	}
	return out
}

func paramsFrom(raw map[string]any, node any) []paramInfo {
	arr, _ := node.([]any)
	out := make([]paramInfo, 0, len(arr))
	for _, p := range arr {
		pm, _ := p.(map[string]any)
		if pm == nil {
			continue
		}
		if ref, ok := pm["$ref"].(string); ok {
			if resolved := resolveRef(raw, ref); resolved != nil {
				pm = resolved
			}
		}
		name, _ := pm["name"].(string)
		in, _ := pm["in"].(string)
		required, _ := pm["required"].(bool)
		var schema string
		var expanded []string
		if s, ok := pm["schema"].(map[string]any); ok {
			schema = schemaSummary(s)
			expanded = expandSchema(raw, s, map[string]bool{})
		} else if ref, ok := pm["$ref"].(string); ok {
			schema = "$ref " + refTail(ref)
		}
		out = append(out, paramInfo{name: name, in: in, required: required, schema: schema, expanded: expanded})
	}
	return out
}

func bodiesFromContent(raw map[string]any, node any) []bodyInfo {
	content, _ := node.(map[string]any)
	if content == nil {
		return nil
	}
	cts := make([]string, 0, len(content))
	for ct := range content {
		cts = append(cts, ct)
	}
	sort.Strings(cts)
	out := make([]bodyInfo, 0, len(cts))
	for _, ct := range cts {
		m, _ := content[ct].(map[string]any)
		var schema string
		var expanded []string
		if m != nil {
			if s, ok := m["schema"].(map[string]any); ok {
				schema = schemaSummary(s)
				expanded = expandSchema(raw, s, map[string]bool{})
			}
		}
		out = append(out, bodyInfo{contentType: ct, schema: schema, expanded: expanded})
	}
	return out
}

func resolveRef(raw map[string]any, ref string) map[string]any {
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	var cur any = raw
	for _, p := range parts {
		p = strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~")
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	out, _ := cur.(map[string]any)
	return out
}

// expandSchema returns properties of the given schema one level deep. $refs
// are resolved once (cycle-safe via seen set) and shown as nested property
// lines; objects list their properties; arrays show their item summary.
func expandSchema(raw, schema map[string]any, seen map[string]bool) []string {
	if schema == nil {
		return nil
	}
	if ref, ok := schema["$ref"].(string); ok {
		if seen[ref] {
			return nil
		}
		seen[ref] = true
		resolved := resolveRef(raw, ref)
		return expandSchema(raw, resolved, seen)
	}
	t, _ := schema["type"].(string)
	if t == "array" {
		items, _ := schema["items"].(map[string]any)
		return expandSchema(raw, items, seen)
	}
	if t == "object" || schema["properties"] != nil {
		return expandProps(raw, schema)
	}
	return nil
}

func expandProps(raw, schema map[string]any) []string {
	props, _ := schema["properties"].(map[string]any)
	if len(props) == 0 {
		return nil
	}
	required, _ := schema["required"].([]any)
	reqSet := map[string]bool{}
	for _, r := range required {
		if s, ok := r.(string); ok {
			reqSet[s] = true
		}
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	reqStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	max := 0
	for _, k := range keys {
		if len(k) > max {
			max = len(k)
		}
	}
	if max > 28 {
		max = 28
	}

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		sub, _ := props[k].(map[string]any)
		marker := " "
		if reqSet[k] {
			marker = reqStyle.Render("*")
		}
		out = append(out, fmt.Sprintf("%s %s  %s", marker, nameStyle.Render(padr(k, max)), dim.Render(schemaSummary(sub))))
	}
	return out
}

func schemaSummary(schema map[string]any) string {
	if schema == nil {
		return ""
	}
	if ref, ok := schema["$ref"].(string); ok {
		return "$ref " + refTail(ref)
	}
	for _, key := range []string{"oneOf", "anyOf", "allOf"} {
		if arr, ok := schema[key].([]any); ok && len(arr) > 0 {
			return fmt.Sprintf("%s[%d]", key, len(arr))
		}
	}
	t, _ := schema["type"].(string)
	switch t {
	case "array":
		items, _ := schema["items"].(map[string]any)
		return "array<" + schemaSummary(items) + ">"
	case "object":
		if props, ok := schema["properties"].(map[string]any); ok {
			return fmt.Sprintf("object{%d props}", len(props))
		}
		return "object"
	case "":
		if enum, ok := schema["enum"].([]any); ok {
			return fmt.Sprintf("enum[%d]", len(enum))
		}
		return "?"
	default:
		if format, _ := schema["format"].(string); format != "" {
			return t + "(" + format + ")"
		}
		return t
	}
}

func refTail(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}

func codeStyle(code string) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(true)
	switch {
	case strings.HasPrefix(code, "2"):
		return base.Foreground(lipgloss.Color("42"))
	case strings.HasPrefix(code, "3"):
		return base.Foreground(lipgloss.Color("39"))
	case strings.HasPrefix(code, "4"):
		return base.Foreground(lipgloss.Color("214"))
	case strings.HasPrefix(code, "5"):
		return base.Foreground(lipgloss.Color("196"))
	default:
		return base.Foreground(lipgloss.Color("244"))
	}
}

func padr(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func (m model) renderTagPicker(width, height int) string {
	if len(m.tags) == 0 {
		return lipgloss.NewStyle().Width(width).Height(height).Render(
			lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("(no tags in this spec)"),
		)
	}
	heading := lipgloss.NewStyle().Bold(true).Render("Tag filter")
	visible := height - 2
	if visible < 1 {
		visible = 1
	}
	start := m.tagCursor - visible/2
	if start+visible > len(m.tags) {
		start = len(m.tags) - visible
	}
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(m.tags) {
		end = len(m.tags)
	}

	var lines []string
	for i := start; i < end; i++ {
		tag := m.tags[i]
		mark := "○"
		if m.tagFilter[tag] {
			mark = "●"
		}
		cursor := "  "
		if i == m.tagCursor {
			cursor = "> "
		}
		fg := lipgloss.Color("250")
		bold := false
		if m.tagFilter[tag] {
			fg = lipgloss.Color("42")
			bold = true
		}
		style := lipgloss.NewStyle().Foreground(fg).Bold(bold)
		row := fmt.Sprintf("%s%s %s", cursor, mark, tag)
		if i == m.tagCursor {
			style = style.Background(lipgloss.Color("236")).Width(width)
		}
		lines = append(lines, style.Render(row))
	}
	for len(lines) < visible {
		lines = append(lines, "")
	}
	return lipgloss.JoinVertical(lipgloss.Left, heading, "", strings.Join(lines, "\n"))
}

func (m model) renderHelp() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")).Render("openapi-extract — keybindings")
	rows := []string{
		title,
		"",
		row("↑/↓ or j/k", "move cursor"),
		row("PgUp/PgDn or ctrl-u/d", "page"),
		row("g / G", "jump to top / bottom"),
		row("space", "toggle selection"),
		row("a / A", "select all visible / clear selection"),
		row("S", "toggle selected-only view"),
		row("/", "live search (enter applies, esc clears)"),
		row("m", "method filter mode"),
		row("t", "tag filter mode"),
		row("e", "toggle drop-examples"),
		row("i", "toggle strip-info-description"),
		row("F", "toggle output format (yaml/json)"),
		row("f", "toggle detail pane fullscreen (wide terminals only)"),
		row("tab / shift-tab", "focus detail pane (j/k scrolls, tab back)"),
		row("mouse wheel", "scroll list / tag picker"),
		row("c", "copy selection to clipboard"),
		row("s", "save selection to file"),
		row("y", "print selection to stdout and quit"),
		row("?", "this help"),
		row("q / ctrl-c", "quit"),
		"",
		dim.Render("In method-filter mode:"),
		row("  g/p/u/a/d/o/h/t", "toggle GET/POST/PUT/PATCH/DELETE/OPTIONS/HEAD/TRACE"),
		row("  c", "clear method filter"),
		row("  esc / enter / m", "return to list"),
		"",
		dim.Render("In tag-filter mode:"),
		row("  j/k or ↑/↓", "move"),
		row("  space", "toggle tag"),
		row("  c", "clear tag filter"),
		row("  esc / enter / t", "return to list"),
		"",
		dim.Render("press any key to return"),
	}
	return strings.Join(rows, "\n")
}

func row(keys, desc string) string {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Width(28)
	return keyStyle.Render(keys) + lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(desc)
}

func (m model) renderFooter() string {
	modeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("228")).Background(lipgloss.Color("238")).Padding(0, 1)
	optStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("147"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	left := modeStyle.Render(m.modeLabel())
	var optParts []string
	optParts = append(optParts, "fmt:"+m.outputFormat)
	if m.extractOpts.DropExamples {
		optParts = append(optParts, "no-examples")
	}
	if m.extractOpts.StripInfoDescription {
		optParts = append(optParts, "no-info-desc")
	}
	if m.showSelectedOnly {
		optParts = append(optParts, "selected-only")
	}
	if m.fullscreen {
		optParts = append(optParts, "full")
	}
	opts := optStyle.Render(strings.Join(optParts, " "))

	statusRow := lipgloss.JoinHorizontal(lipgloss.Left, left, " ", opts, "  ", dim.Render(m.message))
	keyBar := renderKeyBar(m.keyHints(), m.width)
	return statusRow + "\n" + keyBar
}

type keyHint struct{ key, label string }

func (m model) keyHints() []keyHint {
	switch m.mode {
	case modeSearch:
		return []keyHint{{"type", "filter live"}, {"enter", "apply"}, {"esc", "clear"}}
	case modeSave:
		return []keyHint{{"type", "path"}, {"enter", "save"}, {"esc", "cancel"}}
	case modeMethodFilter:
		return []keyHint{{"g/p/u/a/d/o/h/t", "toggle"}, {"c", "clear"}, {"esc", "done"}}
	case modeTagFilter:
		return []keyHint{{"j/k", "move"}, {"space", "toggle"}, {"c", "clear"}, {"esc", "done"}}
	case modeHelp:
		return []keyHint{{"any", "back"}}
	}
	if m.detailFocus {
		return []keyHint{
			{"j/k", "scroll"}, {"PgUp/PgDn", "page"},
			{"tab", "list"}, {"f", "full"}, {"q", "quit"},
		}
	}
	return []keyHint{
		{"space", "sel"}, {"a/A", "all/none"}, {"S", "only"},
		{"/", "find"}, {"m", "method"}, {"t", "tag"},
		{"tab", "detail"}, {"f", "full"},
		{"e/i", "opts"}, {"F", "fmt"},
		{"c", "copy"}, {"s", "save"}, {"y", "stdout"},
		{"?", "help"}, {"q", "quit"},
	}
}

func renderKeyBar(hints []keyHint, width int) string {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("228")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(" · ")
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, keyStyle.Render("<"+h.key+">")+" "+descStyle.Render(h.label))
	}
	bar := strings.Join(parts, sep)
	if width > 0 {
		bar = lipgloss.NewStyle().MaxWidth(width).Render(bar)
	}
	return bar
}

func (m model) modeLabel() string {
	switch m.mode {
	case modeSearch:
		return "[I]"
	case modeSave:
		return "[W]"
	case modeMethodFilter:
		return "[M]"
	case modeTagFilter:
		return "[T]"
	case modeHelp:
		return "[?]"
	default:
		return "[N]"
	}
}

func fieldLine(name, value string) string {
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Bold(true)
	return nameStyle.Render(name+":") + " " + value
}

func wrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	var lines []string
	for _, paragraph := range strings.Split(s, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		cur := words[0]
		for _, w := range words[1:] {
			if len(cur)+1+len(w) > width {
				lines = append(lines, cur)
				cur = w
				continue
			}
			cur += " " + w
		}
		lines = append(lines, cur)
	}
	return strings.Join(lines, "\n")
}

func (m model) listHeight() int {
	h := m.height - 5
	if m.mode == modeSearch || m.mode == modeSave || m.mode == modeMethodFilter {
		h--
	}
	if len(m.activeChips()) > 0 && m.mode != modeSearch {
		h--
	}
	if h < 1 {
		h = 1
	}
	return h
}

func uniqueTags(ops []catalog.Operation) []string {
	seen := map[string]bool{}
	var out []string
	for _, op := range ops {
		for _, t := range op.Tags {
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	sort.Strings(out)
	return out
}

func methodSet(ops []catalog.Operation) map[string]bool {
	out := map[string]bool{}
	for _, op := range ops {
		out[strings.ToUpper(op.Method)] = true
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func keyForMethod(method string) string {
	for k, v := range methodKey {
		if v == method {
			return k
		}
	}
	return ""
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
