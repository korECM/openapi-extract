package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/devsisters/openapi-extract/internal/catalog"
	"github.com/devsisters/openapi-extract/internal/extractor"
	"github.com/devsisters/openapi-extract/internal/output"
	"github.com/devsisters/openapi-extract/internal/specio"
	"github.com/devsisters/openapi-extract/internal/tui"
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
		fmt.Fprintln(stderr, "usage: openapi-extract list <openapi.yaml|-> [--format text|json]")
		return 2
	}
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "output format: text or json")
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
	ops := catalog.Build(loaded.Doc)
	switch *format {
	case "json":
		data, err := output.Marshal(ops, "json")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		stdout.Write(data)
	case "text", "":
		for _, op := range ops {
			summary := op.Summary
			if summary != "" {
				summary = " - " + summary
			}
			fmt.Fprintf(stdout, "%s\t%s %s%s\n", op.ID, op.Method, op.Path, summary)
		}
	default:
		fmt.Fprintf(stderr, "unsupported format: %s\n", *format)
		return 2
	}
	return 0
}

func runExtract(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	source, flagArgs, ok := splitSource(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: openapi-extract extract <openapi.yaml|-> (--id ID|--select 'METHOD /path') (--stdout|--copy|--output file)")
		return 2
	}
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var ids repeated
	var selects repeated
	fs.Var(&ids, "id", "operation id from `list` output; may be repeated")
	fs.Var(&selects, "select", "operation selector like `GET /path`; may be repeated")
	format := fs.String("format", "yaml", "output format: yaml or json")
	toStdout := fs.Bool("stdout", false, "write mini spec to stdout")
	toCopy := fs.Bool("copy", false, "copy mini spec to clipboard")
	outputPath := fs.String("output", "", "write mini spec to file")
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected arguments:", strings.Join(fs.Args(), " "))
		return 2
	}
	if len(ids) == 0 && len(selects) == 0 {
		fmt.Fprintln(stderr, "missing operation selection: use --id or --select")
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
	selected, err := catalog.Find(ops, ids, selects)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	mini, err := extractor.Extract(loaded.Raw, selected)
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
  openapi-extract <openapi.yaml|->                 Open interactive TUI
  openapi-extract list <openapi.yaml|->            Print operation catalog
  openapi-extract extract <openapi.yaml|->          Extract selected operations

AI-friendly flow:
  openapi-extract list openapi.yaml --format json
  openapi-extract extract openapi.yaml --id 'get_/players/{player_id}' --stdout
`))
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
