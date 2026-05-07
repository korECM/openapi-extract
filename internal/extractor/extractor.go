package extractor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devsisters/openapi-extract/internal/catalog"
)

func Extract(raw map[string]any, selected []catalog.Operation) (map[string]any, error) {
	if len(selected) == 0 {
		return nil, fmt.Errorf("no operations selected")
	}

	mini := map[string]any{}
	copyRoot(mini, raw, "openapi")
	copyRoot(mini, raw, "info")
	copyRoot(mini, raw, "jsonSchemaDialect")
	copyRoot(mini, raw, "externalDocs")
	copyRoot(mini, raw, "servers")
	copyRoot(mini, raw, "security")

	paths, ok := asMap(raw["paths"])
	if !ok {
		return nil, fmt.Errorf("OpenAPI document has no paths object")
	}

	miniPaths := map[string]any{}
	for _, op := range selected {
		pathItem, ok := asMap(paths[op.Path])
		if !ok {
			return nil, fmt.Errorf("operation not found: %s", op.ID)
		}
		method := strings.ToLower(op.Method)
		operation, ok := pathItem[method]
		if !ok {
			return nil, fmt.Errorf("operation not found: %s", op.ID)
		}

		target, _ := asMap(miniPaths[op.Path])
		if target == nil {
			target = map[string]any{}
			miniPaths[op.Path] = target
		}
		for _, field := range []string{"summary", "description", "servers", "parameters"} {
			if value, exists := pathItem[field]; exists {
				target[field] = value
			}
		}
		target[method] = operation
	}
	mini["paths"] = miniPaths

	filterTags(mini, raw, selected)
	if err := includeReachableComponents(mini, raw); err != nil {
		return nil, err
	}
	return mini, nil
}

func copyRoot(dst, src map[string]any, key string) {
	if value, ok := src[key]; ok {
		dst[key] = value
	}
}

func filterTags(mini, raw map[string]any, selected []catalog.Operation) {
	rawTags, ok := raw["tags"].([]any)
	if !ok {
		return
	}
	needed := map[string]bool{}
	for _, op := range selected {
		for _, tag := range op.Tags {
			needed[tag] = true
		}
	}
	if len(needed) == 0 {
		return
	}
	tags := make([]any, 0, len(rawTags))
	for _, item := range rawTags {
		tag, ok := asMap(item)
		if !ok {
			continue
		}
		name, _ := tag["name"].(string)
		if needed[name] {
			tags = append(tags, item)
		}
	}
	if len(tags) > 0 {
		mini["tags"] = tags
	}
}

func includeReachableComponents(mini, raw map[string]any) error {
	rawComponents, ok := asMap(raw["components"])
	if !ok {
		return nil
	}
	miniComponents := map[string]any{}
	queue := refsIn(mini)
	seen := map[string]bool{}
	if err := includeSecuritySchemes(miniComponents, rawComponents, mini); err != nil {
		return err
	}

	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		if seen[ref] {
			continue
		}
		seen[ref] = true

		section, name, ok := parseComponentRef(ref)
		if !ok {
			continue
		}
		rawSection, ok := asMap(rawComponents[section])
		if !ok {
			return fmt.Errorf("unresolved reference: %s", ref)
		}
		value, ok := rawSection[name]
		if !ok {
			return fmt.Errorf("unresolved reference: %s", ref)
		}

		targetSection, _ := asMap(miniComponents[section])
		if targetSection == nil {
			targetSection = map[string]any{}
			miniComponents[section] = targetSection
		}
		targetSection[name] = value
		queue = append(queue, refsIn(value)...)
	}

	if len(miniComponents) > 0 {
		mini["components"] = sortComponentSections(miniComponents)
	}
	return nil
}

func includeSecuritySchemes(miniComponents, rawComponents map[string]any, mini map[string]any) error {
	names := securitySchemeNames(mini)
	if len(names) == 0 {
		return nil
	}
	rawSchemes, ok := asMap(rawComponents["securitySchemes"])
	if !ok {
		return nil
	}
	target := map[string]any{}
	for name := range names {
		value, ok := rawSchemes[name]
		if !ok {
			return fmt.Errorf("unresolved security scheme: %s", name)
		}
		target[name] = value
	}
	if len(target) > 0 {
		miniComponents["securitySchemes"] = target
	}
	return nil
}

func securitySchemeNames(value any) map[string]bool {
	names := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case map[string]any:
			if security, ok := typed["security"]; ok {
				collectSecurityNames(names, security)
			}
			for key, child := range typed {
				if key == "security" {
					continue
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return names
}

func collectSecurityNames(names map[string]bool, value any) {
	requirements, ok := value.([]any)
	if !ok {
		return
	}
	for _, item := range requirements {
		requirement, ok := asMap(item)
		if !ok {
			continue
		}
		for name := range requirement {
			names[name] = true
		}
	}
}

func refsIn(value any) []string {
	var refs []string
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case map[string]any:
			if ref, ok := typed["$ref"].(string); ok {
				refs = append(refs, ref)
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return refs
}

func parseComponentRef(ref string) (string, string, bool) {
	const prefix = "#/components/"
	if !strings.HasPrefix(ref, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(ref, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	return unescapePointer(parts[0]), unescapePointer(parts[1]), true
}

func unescapePointer(value string) string {
	value = strings.ReplaceAll(value, "~1", "/")
	value = strings.ReplaceAll(value, "~0", "~")
	return value
}

func asMap(value any) (map[string]any, bool) {
	typed, ok := value.(map[string]any)
	return typed, ok
}

func sortComponentSections(components map[string]any) map[string]any {
	ordered := map[string]any{}
	keys := make([]string, 0, len(components))
	for key := range components {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		ordered[key] = components[key]
	}
	return ordered
}
