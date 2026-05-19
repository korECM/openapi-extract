package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/korECM/openapi-extract/internal/catalog"
	"github.com/korECM/openapi-extract/internal/extractor"
	"github.com/korECM/openapi-extract/internal/output"
	"github.com/korECM/openapi-extract/internal/specio"
	"github.com/korECM/openapi-extract/internal/tui"
)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		return runList(args[1:], stdin, stdout, stderr)
	case "extract":
		return runExtract(args[1:], stdin, stdout, stderr)
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	default:
		return runTUI(args, stdin, stdout, stderr)
	}
}

func runList(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	source, flagArgs, ok := splitSource(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: openapi-extract list <openapi.yaml|url|-> [--format text|json]")
		return 2
	}
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "output format: text or json")
	columnsFlag := fs.String("columns", "id,method,path,summary", "comma-separated text columns: id,method,path,operationId,summary,tags or all")
	noHeader := fs.Bool("no-header", false, "hide the header row in text output")
	noColor := fs.Bool("no-color", false, "disable ANSI colors in text output")
	var tagFilter repeated
	fs.Var(&tagFilter, "tag", "filter operations by tag (case-insensitive, repeatable, OR semantics)")
	maxColWidth := fs.Int("max-col-width", 0, "truncate text-column cells to N runes with ellipsis (0 = no limit)")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected arguments:", strings.Join(fs.Args(), " "))
		return 2
	}

	loaded, err := specio.Load(source, stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ops := catalog.FilterByTags(catalog.Build(loaded.Doc), tagFilter)
	switch *format {
	case "json":
		data, err := output.Marshal(ops, "json")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		stdout.Write(data)
	case "text", "":
		columns, err := parseColumns(*columnsFlag)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		writeTextCatalog(stdout, ops, columns, !*noHeader, shouldColor(stdout, *noColor), *maxColWidth)
	default:
		fmt.Fprintf(stderr, "unsupported format: %s\n", *format)
		return 2
	}
	return 0
}

func writeTextCatalog(w io.Writer, ops []catalog.Operation, columns []listColumn, header bool, color bool, maxColWidth int) {
	values := make([][]string, len(ops))
	widths := make([]int, len(columns))
	for i, column := range columns {
		widths[i] = runeLen(column.Header)
	}
	for opIdx, op := range ops {
		row := make([]string, len(columns))
		for i, column := range columns {
			cell := truncateCell(column.Value(op), maxColWidth)
			row[i] = cell
			widths[i] = max(widths[i], runeLen(cell))
		}
		values[opIdx] = row
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250"))
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	if header {
		cells := make([]string, 0, len(columns))
		for i, column := range columns {
			cell := column.Header
			if i < len(columns)-1 {
				cell = padRight(cell, widths[i])
			}
			if color {
				cell = headerStyle.Render(cell)
			}
			cells = append(cells, cell)
		}
		fmt.Fprintln(w, strings.Join(cells, "  "))
	}

	for opIdx, op := range ops {
		cells := make([]string, 0, len(columns))
		for i, column := range columns {
			value := values[opIdx][i]
			if i < len(columns)-1 {
				value = padRight(value, widths[i])
			}
			if color {
				switch column.Name {
				case "id":
					value = idStyle.Render(value)
				case "method":
					value = methodStyle(op.Method).Render(value)
				case "path":
					value = pathStyle.Render(value)
				case "summary", "operationId", "tags":
					value = summaryStyle.Render(value)
				}
			}
			cells = append(cells, value)
		}
		fmt.Fprintln(w, strings.Join(cells, "  "))
	}
}

type listColumn struct {
	Name   string
	Header string
	Value  func(catalog.Operation) string
}

func parseColumns(value string) ([]listColumn, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("missing columns")
	}
	if value == "all" {
		value = "id,method,path,operationId,summary,tags"
	}
	parts := strings.Split(value, ",")
	columns := make([]listColumn, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		column, ok := knownListColumns[name]
		if !ok {
			return nil, fmt.Errorf("unknown column %q; available columns: id,method,path,operationId,summary,tags", name)
		}
		columns = append(columns, column)
	}
	return columns, nil
}

var knownListColumns = map[string]listColumn{
	"id": {
		Name:   "id",
		Header: "ID",
		Value:  func(op catalog.Operation) string { return op.ID },
	},
	"method": {
		Name:   "method",
		Header: "METHOD",
		Value:  func(op catalog.Operation) string { return op.Method },
	},
	"path": {
		Name:   "path",
		Header: "PATH",
		Value:  func(op catalog.Operation) string { return op.Path },
	},
	"operationId": {
		Name:   "operationId",
		Header: "OPERATION ID",
		Value:  func(op catalog.Operation) string { return op.OperationID },
	},
	"summary": {
		Name:   "summary",
		Header: "SUMMARY",
		Value:  func(op catalog.Operation) string { return op.Summary },
	},
	"tags": {
		Name:   "tags",
		Header: "TAGS",
		Value:  func(op catalog.Operation) string { return strings.Join(op.Tags, ",") },
	},
}

func padRight(value string, width int) string {
	current := runeLen(value)
	if current >= width {
		return value
	}
	return value + strings.Repeat(" ", width-current)
}

func runeLen(value string) int {
	return len([]rune(value))
}

func truncateCell(value string, maxWidth int) string {
	if maxWidth <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxWidth {
		return value
	}
	if maxWidth == 1 {
		return "…"
	}
	return string(runes[:maxWidth-1]) + "…"
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

func shouldColor(w io.Writer, noColor bool) bool {
	if noColor || os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func runExtract(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	source, flagArgs, ok := splitSource(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: openapi-extract extract <openapi.yaml|url|-> (--id ID|--select 'METHOD /path') (--stdout|--copy|--output file)")
		return 2
	}
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var ids repeated
	var selects repeated
	var tagSelect repeated
	fs.Var(&ids, "id", "operation id from `list` output; may be repeated")
	fs.Var(&selects, "select", "operation selector like `GET /path`; may be repeated")
	fs.Var(&tagSelect, "tag", "select all operations carrying this tag (case-insensitive, repeatable)")
	format := fs.String("format", "yaml", "output format: yaml or json")
	toStdout := fs.Bool("stdout", false, "write mini spec to stdout")
	toCopy := fs.Bool("copy", false, "copy mini spec to clipboard")
	outputPath := fs.String("output", "", "write mini spec to file")
	stripInfo := fs.Bool("strip-info-description", false, "drop info.description from mini spec")
	keepInfo := fs.Bool("keep-info-description", false, "keep info.description even when --stdout default would strip it")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected arguments:", strings.Join(fs.Args(), " "))
		return 2
	}
	if len(ids) == 0 && len(selects) == 0 && len(tagSelect) == 0 {
		fmt.Fprintln(stderr, "missing operation selection: use --id, --select, or --tag")
		return 2
	}
	if !*toStdout && !*toCopy && *outputPath == "" {
		fmt.Fprintln(stderr, "missing output target: use --stdout, --copy, or --output")
		return 2
	}

	loaded, err := specio.Load(source, stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ops := catalog.Build(loaded.Doc)
	var result catalog.FindResult
	if len(ids) > 0 || len(selects) > 0 {
		result, err = catalog.Find(ops, ids, selects)
		if err != nil && len(tagSelect) == 0 {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, m := range result.Missing {
			fmt.Fprintf(stderr, "warning: operation not found: %s\n", m)
		}
	}
	if len(tagSelect) > 0 {
		seen := map[string]bool{}
		for _, op := range result.Operations {
			seen[op.ID] = true
		}
		taggedAdded := 0
		for _, op := range catalog.FilterByTags(ops, tagSelect) {
			if seen[op.ID] {
				continue
			}
			seen[op.ID] = true
			result.Operations = append(result.Operations, op)
			taggedAdded++
		}
		if taggedAdded == 0 && len(result.Operations) == 0 {
			fmt.Fprintf(stderr, "no operations matched the requested tags: %s\n", strings.Join(tagSelect, ", "))
			return 1
		}
	}
	if len(result.Operations) == 0 {
		fmt.Fprintln(stderr, "no operations matched")
		return 1
	}
	opts := extractor.Options{StripInfoDescription: resolveStripInfo(*stripInfo, *keepInfo, *toStdout)}
	mini, err := extractor.Extract(loaded.Raw, result.Operations, opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	data, err := output.Marshal(mini, *format)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if *toStdout {
		stdout.Write(data)
	}
	if *toCopy {
		if err := output.Copy(data); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if *outputPath != "" {
		if err := output.WriteFile(*outputPath, data); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	return 0
}

func runTUI(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		usage(stderr)
		return 2
	}
	loaded, err := specio.Load(args[0], stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := tui.Run(loaded.Raw, catalog.Build(loaded.Doc)); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, strings.TrimSpace(`
Usage:
  openapi-extract <openapi.yaml|url|->             Open interactive TUI
  openapi-extract list <openapi.yaml|url|->        Print operation catalog
  openapi-extract extract <openapi.yaml|url|->      Extract selected operations

AI-friendly flow:
  openapi-extract list openapi.yaml --format json
  openapi-extract extract openapi.yaml --id 'get_/players/{player_id}' --stdout
`))
}

func resolveStripInfo(strip, keep, toStdout bool) bool {
	if keep {
		return false
	}
	if strip {
		return true
	}
	return toStdout
}

type repeated []string

func (r *repeated) String() string {
	return strings.Join(*r, ",")
}

func (r *repeated) Set(value string) error {
	*r = append(*r, value)
	return nil
}

func splitSource(args []string) (string, []string, bool) {
	for i, arg := range args {
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if arg == "--" && i+1 < len(args) {
				flagArgs := append([]string{}, args[:i]...)
				flagArgs = append(flagArgs, args[i+2:]...)
				return args[i+1], flagArgs, true
			}
			continue
		}
		flagArgs := append([]string{}, args[:i]...)
		flagArgs = append(flagArgs, args[i+1:]...)
		return arg, flagArgs, true
	}
	return "", nil, false
}
