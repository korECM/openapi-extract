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

func Find(ops []Operation, ids []string, selects []string) ([]Operation, error) {
	byID := make(map[string]Operation, len(ops))
	bySelect := make(map[string]Operation, len(ops))
	for _, op := range ops {
		byID[op.ID] = op
		bySelect[selectorKey(op.Method, op.Path)] = op
	}

	selected := make([]Operation, 0, len(ids)+len(selects))
	seen := map[string]bool{}
	for _, id := range ids {
		op, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("operation not found: %s", id)
		}
		if !seen[op.ID] {
			selected = append(selected, op)
			seen[op.ID] = true
		}
	}
	for _, selector := range selects {
		op, ok := bySelect[selectorKeyFromString(selector)]
		if !ok {
			return nil, fmt.Errorf("operation not found: %s", selector)
		}
		if !seen[op.ID] {
			selected = append(selected, op)
			seen[op.ID] = true
		}
	}
	return selected, nil
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
