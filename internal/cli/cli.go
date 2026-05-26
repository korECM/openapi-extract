package cli

import (
	"encoding/json"
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

// RunOptions carries runtime data injected from main, primarily the resolved
// binary version reported by --version.
type RunOptions struct {
	Version string
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, runOpts ...RunOptions) int {
	var opts RunOptions
	if len(runOpts) > 0 {
		opts = runOpts[0]
	}

	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: missing OpenAPI source")
		fmt.Fprintln(stderr)
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
	case "-v", "--version", "version":
		fmt.Fprintln(stdout, displayVersion(opts.Version))
		return 0
	default:
		return runTUI(args, stdin, stdout, stderr)
	}
}

func displayVersion(v string) string {
	if v == "" {
		return "(devel)"
	}
	return v
}

func runList(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if helpRequested(args) {
		writeListHelp(stdout)
		return 0
	}
	source, flagArgs, ok := splitSource(args)
	if !ok {
		writeListHelp(stderr)
		return 2
	}
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	format := fs.String("format", "text", "output format: text, tree, or json")
	columnsFlag := fs.String("columns", "id,method,path,summary", "comma-separated text columns: id,method,path,operationId,summary,description,tags,deprecated,security,body,responseCodes or all")
	noHeader := fs.Bool("no-header", false, "hide the header row in text output")
	noColor := fs.Bool("no-color", false, "disable ANSI colors in text output")
	var tagFilter repeated
	fs.Var(&tagFilter, "tag", "filter operations by tag (case-insensitive, repeatable, OR semantics)")
	var excludeTagFilter repeated
	fs.Var(&excludeTagFilter, "exclude-tag", "drop operations carrying this tag (case-insensitive, repeatable); applied after --tag")
	var methodFilter repeated
	fs.Var(&methodFilter, "method", "filter by HTTP method (case-insensitive, comma-separated or repeatable, OR semantics)")
	pathPrefix := fs.String("path-prefix", "", "keep only operations whose path starts with this prefix (case-sensitive)")
	grep := fs.String("grep", "", "case-insensitive substring match against id, operationId, summary, description")
	noDeprecated := fs.Bool("no-deprecated", false, "drop deprecated operations")
	maxColWidth := fs.Int("max-col-width", 0, "truncate text-column cells to N runes with ellipsis (0 = no limit)")
	noCache := fs.Bool("no-cache", false, "bypass the on-disk URL cache for this request")
	refreshCache := fs.Bool("refresh-cache", false, "ignore the cached URL response and overwrite it")
	verbose := fs.Bool("verbose", false, "log cache hit/miss/304 status for URL fetches to stderr")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected arguments:", strings.Join(fs.Args(), " "))
		return 2
	}

	loaded, err := specio.LoadWithOptions(source, stdin, specio.Options{
		NoCache:      *noCache,
		RefreshCache: *refreshCache,
		CacheLogger:  cacheLogger(stderr, *verbose),
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ops := catalog.Build(loaded.Doc)
	ops = catalog.FilterByTags(ops, tagFilter)
	ops = catalog.FilterExcludeTags(ops, excludeTagFilter)
	ops = catalog.FilterByMethods(ops, methodFilter)
	ops = catalog.FilterByPathPrefix(ops, *pathPrefix)
	ops = catalog.FilterByGrep(ops, *grep)
	ops = catalog.FilterExcludeDeprecated(ops, *noDeprecated)
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
	case "tree":
		writeTreeCatalog(stdout, ops, shouldColor(stdout, *noColor))
	default:
		fmt.Fprintf(stderr, "unsupported format: %s\n", *format)
		return 2
	}
	return 0
}

func cacheLogger(stderr io.Writer, verbose bool) func(string) {
	if !verbose {
		return nil
	}
	return func(msg string) {
		fmt.Fprintln(stderr, "cache:", msg)
	}
}

func writeTreeCatalog(w io.Writer, ops []catalog.Operation, color bool) {
	pathStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	byKey := make(map[string][]catalog.Operation)
	order := make([]string, 0)
	for _, op := range ops {
		key := op.Path
		if op.Kind == catalog.KindWebhook {
			key = "webhook:" + op.Path
		}
		if _, seen := byKey[key]; !seen {
			order = append(order, key)
		}
		byKey[key] = append(byKey[key], op)
	}

	for _, key := range order {
		group := byKey[key]
		header := group[0].Path
		if group[0].Kind == catalog.KindWebhook {
			header = "webhook " + group[0].Path
		}
		if color {
			fmt.Fprintln(w, pathStyle.Render(header))
		} else {
			fmt.Fprintln(w, header)
		}
		for i, op := range group {
			branch := "├─"
			if i == len(group)-1 {
				branch = "└─"
			}
			method := op.Method
			if color {
				method = methodStyle(op.Method).Render(padRight(op.Method, 6))
			} else {
				method = padRight(op.Method, 6)
			}
			summary := op.Summary
			if summary == "" {
				summary = op.OperationID
			}
			if color {
				summary = summaryStyle.Render(summary)
				branch = dimStyle.Render(branch)
			}
			fmt.Fprintf(w, "%s %s  %s\n", branch, method, summary)
		}
	}
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
		value = "id,method,path,operationId,summary,description,tags,deprecated,security,body,responseCodes"
	}
	parts := strings.Split(value, ",")
	columns := make([]listColumn, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		column, ok := knownListColumns[name]
		if !ok {
			return nil, fmt.Errorf("unknown column %q; available columns: id,method,path,operationId,summary,description,tags,deprecated,security,body,responseCodes", name)
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
	"description": {
		Name:   "description",
		Header: "DESCRIPTION",
		Value:  func(op catalog.Operation) string { return op.Description },
	},
	"deprecated": {
		Name:   "deprecated",
		Header: "DEPRECATED",
		Value: func(op catalog.Operation) string {
			if op.Deprecated {
				return "yes"
			}
			return ""
		},
	},
	"security": {
		Name:   "security",
		Header: "SECURITY",
		Value:  func(op catalog.Operation) string { return strings.Join(op.Security, ",") },
	},
	"body": {
		Name:   "body",
		Header: "BODY",
		Value: func(op catalog.Operation) string {
			switch {
			case !op.HasRequestBody:
				return ""
			case op.RequestBodyRequired:
				return "required"
			default:
				return "optional"
			}
		},
	},
	"responseCodes": {
		Name:   "responseCodes",
		Header: "RESPONSES",
		Value:  func(op catalog.Operation) string { return strings.Join(op.ResponseCodes, ",") },
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
	if helpRequested(args) {
		writeExtractHelp(stdout)
		return 0
	}
	source, flagArgs, ok := splitSource(args)
	if !ok {
		writeExtractHelp(stderr)
		return 2
	}
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	var ids repeated
	var selects repeated
	var tagSelect repeated
	var excludeTagFilter repeated
	var methodFilter repeated
	fs.Var(&ids, "id", "operation id from `list` output; may be repeated")
	fs.Var(&selects, "select", "operation selector like `GET /path`; may be repeated")
	fs.Var(&tagSelect, "tag", "select all operations carrying this tag (case-insensitive, repeatable)")
	fs.Var(&excludeTagFilter, "exclude-tag", "drop operations carrying this tag from the final selection (case-insensitive, repeatable)")
	fs.Var(&methodFilter, "method", "narrow the final selection to operations matching this HTTP method (case-insensitive, comma-separated or repeatable)")
	pathPrefix := fs.String("path-prefix", "", "narrow the final selection to operations whose path starts with this prefix")
	grep := fs.String("grep", "", "narrow the final selection by case-insensitive substring against id, operationId, summary, description")
	noDeprecated := fs.Bool("no-deprecated", false, "drop deprecated operations from the final selection")
	format := fs.String("format", "yaml", "output format: yaml or json")
	toStdout := fs.Bool("stdout", false, "write mini spec to stdout")
	toCopy := fs.Bool("copy", false, "copy mini spec to clipboard")
	outputPath := fs.String("output", "", "write mini spec to file")
	stripInfo := fs.Bool("strip-info-description", false, "drop info.description from mini spec (default)")
	keepInfo := fs.Bool("keep-info-description", false, "keep info.description in mini spec")
	maxEnum := fs.Int("max-enum", 0, "truncate JSON Schema enum arrays longer than N entries (0 = unlimited)")
	dropExamples := fs.Bool("drop-examples", false, "remove example and examples fields at every level")
	noCache := fs.Bool("no-cache", false, "bypass the on-disk URL cache for this request")
	refreshCache := fs.Bool("refresh-cache", false, "ignore the cached URL response and overwrite it")
	quiet := fs.Bool("quiet", false, "suppress the size-summary line normally printed to stderr on success")
	verbose := fs.Bool("verbose", false, "log cache hit/miss/304 status for URL fetches to stderr")
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

	loaded, err := specio.LoadWithOptions(source, stdin, specio.Options{
		NoCache:      *noCache,
		RefreshCache: *refreshCache,
		CacheLogger:  cacheLogger(stderr, *verbose),
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	jsonWarn := *format == "json"
	ops := catalog.Build(loaded.Doc)
	var result catalog.FindResult
	if len(ids) > 0 || len(selects) > 0 {
		result, err = catalog.Find(ops, ids, selects)
		if err != nil && len(tagSelect) == 0 {
			fmt.Fprintln(stderr, err)
			return 1
		}
		for _, m := range result.Missing {
			emitWarning(stderr, jsonWarn, "missing_id", m)
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
	// Post-selection filters narrow the chosen set with AND semantics. This
	// gives `extract` the same orthogonality as `list` so e.g. "all of Orders
	// tag, but only GET" stays a single command.
	before := len(result.Operations)
	result.Operations = catalog.FilterExcludeTags(result.Operations, excludeTagFilter)
	result.Operations = catalog.FilterByMethods(result.Operations, methodFilter)
	result.Operations = catalog.FilterByPathPrefix(result.Operations, *pathPrefix)
	result.Operations = catalog.FilterByGrep(result.Operations, *grep)
	result.Operations = catalog.FilterExcludeDeprecated(result.Operations, *noDeprecated)
	if before > 0 && len(result.Operations) == 0 {
		fmt.Fprintln(stderr, "no operations matched after applying --method/--path-prefix/--grep/--exclude-tag/--no-deprecated filters")
		return 1
	}
	if len(result.Operations) == 0 {
		fmt.Fprintln(stderr, "no operations matched")
		return 1
	}
	opts := extractor.Options{
		StripInfoDescription: resolveStripInfo(*stripInfo, *keepInfo),
		MaxEnum:              *maxEnum,
		DropExamples:         *dropExamples,
	}
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
	if !*quiet {
		emitSummary(stderr, jsonWarn, len(result.Operations), countSchemas(mini), loaded.RawBytes, len(data))
	}
	return 0
}

func emitWarning(stderr io.Writer, asJSON bool, kind, value string) {
	if asJSON {
		payload := map[string]string{"level": "warn", "kind": kind, "value": value}
		if encoded, err := json.Marshal(payload); err == nil {
			fmt.Fprintln(stderr, string(encoded))
			return
		}
	}
	fmt.Fprintf(stderr, "warning: operation not found: %s\n", value)
}

func emitSummary(stderr io.Writer, asJSON bool, ops, schemas, originalBytes, miniBytes int) {
	reduction := 0.0
	if originalBytes > 0 {
		reduction = (1.0 - float64(miniBytes)/float64(originalBytes)) * 100.0
		if reduction < 0 {
			reduction = 0
		}
	}
	if asJSON {
		payload := map[string]any{
			"level":           "info",
			"kind":            "summary",
			"operations":      ops,
			"schemas":         schemas,
			"originalBytes":   originalBytes,
			"miniBytes":       miniBytes,
			"reductionPct":    int(reduction + 0.5),
		}
		if encoded, err := json.Marshal(payload); err == nil {
			fmt.Fprintln(stderr, string(encoded))
			return
		}
	}
	fmt.Fprintf(stderr,
		"extracted %d %s / %d %s: %s → %s bytes (%d%% smaller)\n",
		ops, pluralize(ops, "op", "ops"),
		schemas, pluralize(schemas, "schema", "schemas"),
		formatBytes(originalBytes), formatBytes(miniBytes), int(reduction+0.5),
	)
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func formatBytes(n int) string {
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(r))
	}
	return string(out)
}

// countSchemas returns the number of components/schemas entries in the mini
// spec, when present. Used only for the human-readable summary; returns 0 when
// the spec carries no components.
func countSchemas(mini any) int {
	type orderedLike interface {
		Get(string) (any, bool)
	}
	root, ok := mini.(orderedLike)
	if !ok {
		return 0
	}
	components, ok := root.Get("components")
	if !ok {
		return 0
	}
	schemas, ok := components.(orderedLike)
	if !ok {
		return 0
	}
	val, ok := schemas.Get("schemas")
	if !ok {
		return 0
	}
	type lenLike interface {
		Len() int
	}
	if l, ok := val.(lenLike); ok {
		return l.Len()
	}
	if m, ok := val.(map[string]any); ok {
		return len(m)
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
	stdoutData, err := tui.Run(loaded.Raw, catalog.Build(loaded.Doc))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(stdoutData) > 0 {
		stdout.Write(stdoutData)
	}
	return 0
}

func helpRequested(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			return true
		}
	}
	return false
}

func writeListHelp(w io.Writer) {
	fmt.Fprintln(w, strings.TrimSpace(`
Usage:
  openapi-extract list <openapi.yaml|url|-> [flags]

Print the operation catalog for an OpenAPI document.

Flags:
  --format text|tree|json  output format (default text)
  --columns LIST           comma-separated columns: id,method,path,operationId,summary,description,
                           tags,deprecated,security,body,responseCodes (or "all")
  --tag NAME               filter by tag (case-insensitive, repeatable)
  --exclude-tag NAME       drop operations carrying this tag (repeatable; applied after --tag)
  --method NAMES           filter by HTTP method (comma-separated or repeatable, case-insensitive)
  --path-prefix /v1        keep only operations whose path starts with the prefix
  --grep PATTERN           substring match against id, operationId, summary, description (case-insensitive)
  --no-deprecated          drop deprecated operations
  --max-col-width N        truncate text cells to N runes with ellipsis (default 0 = no limit)
  --no-header              hide the header row in text output
  --no-color               disable ANSI colors in text output
  --no-cache               bypass the on-disk URL cache for this request
  --refresh-cache          ignore and overwrite any cached URL response
  --verbose                log cache hit/miss/304 status to stderr

Filters combine with AND across types and OR within each repeatable type.

JSON output schema (one object per operation):
  id, method, path, operationId, summary, description, tags, deprecated,
  security, hasRequestBody, requestBodyRequired, responseCodes, kind

Caching:
  URL fetches are cached under $OPENAPI_EXTRACT_CACHE_DIR (default
  os.UserCacheDir() + /openapi-extract). Repeat fetches send ETag /
  Last-Modified headers so the server can answer 304 Not Modified.

Examples:
  openapi-extract list api.yaml --format json
  openapi-extract list api.yaml --tag Orders --tag Payments
  openapi-extract list api.yaml --exclude-tag Webhooks
  openapi-extract list api.yaml --method GET,POST --path-prefix /v1/orders
  openapi-extract list api.yaml --grep 'refund' --no-deprecated
  openapi-extract list api.yaml --format tree
  curl -s https://example.com/api.yaml | openapi-extract list - --max-col-width 60
`))
}

func writeExtractHelp(w io.Writer) {
	fmt.Fprintln(w, strings.TrimSpace(`
Usage:
  openapi-extract extract <openapi.yaml|url|-> (--id|--select|--tag) (--stdout|--copy|--output FILE) [flags]

Extract a minimal OpenAPI spec containing only the selected operations and
their transitively reachable components.

Selection (at least one required):
  --id ID                  operation id from "list" output; method case-insensitive; repeatable
  --select 'METHOD PATH'   operation selector like "GET /v1/orders"; repeatable
  --tag NAME               pull every operation under the given tag; case-insensitive, repeatable

Post-selection filters (AND, narrow the chosen set):
  --exclude-tag NAME       drop operations carrying this tag (repeatable)
  --method NAMES           keep only matching HTTP methods (comma-separated or repeatable)
  --path-prefix /v1        keep only operations whose path starts with the prefix
  --grep PATTERN           substring match against id, operationId, summary, description
  --no-deprecated          drop deprecated operations

Output target (at least one required):
  --stdout                 write the mini spec to stdout
  --copy                   copy the mini spec to the system clipboard
  --output FILE            write the mini spec to FILE

Other flags:
  --format yaml|json         output format (default yaml)
  --keep-info-description    keep info.description (default: stripped)
  --strip-info-description   explicit form of the default; kept for symmetry
  --max-enum N               truncate enum arrays longer than N (0 = unlimited);
                             writes x-enum-truncated: {kept, total} alongside
  --drop-examples            remove example/examples fields at every depth
  --no-cache                 bypass the on-disk URL cache for this request
  --refresh-cache            ignore and overwrite any cached URL response
  --quiet                    suppress the size-summary line normally printed to stderr
  --verbose                  log cache hit/miss/304 status to stderr

On success, a one-line size summary is written to stderr:
  extracted 3 ops / 14 schemas: 44,250 → 14,837 bytes (66% smaller)
With --format json, the summary and any warnings are emitted as JSONL on stderr.

Examples:
  openapi-extract extract api.yaml --id 'get_/v1/orders' --stdout
  openapi-extract extract api.yaml --tag Orders --output orders.yaml
  openapi-extract extract api.yaml --tag Orders --method GET --stdout
  openapi-extract extract api.yaml --tag Orders --tag Payments --exclude-tag Webhooks --stdout
  openapi-extract extract api.yaml --id 'POST_/v1/orders' --id 'get_/v1/orders/{id}' --stdout
`))
}

func usage(w io.Writer) {
	fmt.Fprintln(w, strings.TrimSpace(`
Usage:
  openapi-extract <openapi.yaml|url|->            Open interactive TUI (requires a source)
  openapi-extract list <openapi.yaml|url|->       Print operation catalog
  openapi-extract extract <openapi.yaml|url|->    Extract selected operations
  openapi-extract --version                       Print version and exit
  openapi-extract --help                          Print this help

Subcommand help:
  openapi-extract list --help
  openapi-extract extract --help

AI-friendly flow:
  openapi-extract list openapi.yaml --format json
  openapi-extract extract openapi.yaml --tag Orders --stdout
`))
}

func resolveStripInfo(strip, keep bool) bool {
	if keep {
		return false
	}
	_ = strip
	return true
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
