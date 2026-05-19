package catalog

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type Operation struct {
	ID          string   `json:"id" yaml:"id"`
	Method      string   `json:"method" yaml:"method"`
	Path        string   `json:"path" yaml:"path"`
	OperationID string   `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Summary     string   `json:"summary,omitempty" yaml:"summary,omitempty"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

var Methods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

func Build(doc *openapi3.T) []Operation {
	paths := doc.Paths.Map()
	pathNames := make([]string, 0, len(paths))
	for path := range paths {
		pathNames = append(pathNames, path)
	}
	sort.Strings(pathNames)

	ops := make([]Operation, 0)
	for _, path := range pathNames {
		item := paths[path]
		for _, method := range Methods {
			op := operationForMethod(item, method)
			if op == nil {
				continue
			}
			ops = append(ops, Operation{
				ID:          IDFor(method, path),
				Method:      strings.ToUpper(method),
				Path:        path,
				OperationID: op.OperationID,
				Summary:     op.Summary,
				Tags:        append([]string(nil), op.Tags...),
			})
		}
	}
	return ops
}

// Find resolves ids and selects against the operation catalog.
//
// Missing entries are collected on FindResult.Missing instead of aborting the
// whole call, so callers can extract the successful subset and warn about the
// rest. An error is returned only when nothing matched.
func Find(ops []Operation, ids []string, selects []string) (FindResult, error) {
	byID := make(map[string]Operation, len(ops))
	bySelect := make(map[string]Operation, len(ops))
	for _, op := range ops {
		byID[op.ID] = op
		bySelect[selectorKey(op.Method, op.Path)] = op
	}

	res := FindResult{}
	seen := map[string]bool{}
	for _, id := range ids {
		op, ok := byID[id]
		if !ok {
			res.Missing = append(res.Missing, id)
			continue
		}
		if !seen[op.ID] {
			res.Operations = append(res.Operations, op)
			seen[op.ID] = true
		}
	}
	for _, selector := range selects {
		op, ok := bySelect[selectorKeyFromString(selector)]
		if !ok {
			res.Missing = append(res.Missing, selector)
			continue
		}
		if !seen[op.ID] {
			res.Operations = append(res.Operations, op)
			seen[op.ID] = true
		}
	}
	if len(res.Operations) == 0 {
		return res, fmt.Errorf("no operations matched (missing: %s)", strings.Join(res.Missing, ", "))
	}
	return res, nil
}

type FindResult struct {
	Operations []Operation
	Missing    []string
}

func selectorKey(method, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(path)
}

func selectorKeyFromString(selector string) string {
	parts := strings.Fields(strings.TrimSpace(selector))
	if len(parts) < 2 {
		return strings.ToUpper(strings.TrimSpace(selector))
	}
	return selectorKey(parts[0], strings.Join(parts[1:], " "))
}

func IDFor(method, path string) string {
	return strings.ToLower(method) + "_" + path
}

func operationForMethod(item *openapi3.PathItem, method string) *openapi3.Operation {
	switch method {
	case "get":
		return item.Get
	case "put":
		return item.Put
	case "post":
		return item.Post
	case "delete":
		return item.Delete
	case "options":
		return item.Options
	case "head":
		return item.Head
	case "patch":
		return item.Patch
	case "trace":
		return item.Trace
	default:
		return nil
	}
}
