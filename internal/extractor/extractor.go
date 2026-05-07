package extractor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/korECM/openapi-extract/internal/catalog"
	"github.com/korECM/openapi-extract/internal/ordered"
)

func Extract(raw map[string]any, selected []catalog.Operation) (*ordered.Map, error) {
	if len(selected) == 0 {
		return nil, fmt.Errorf("no operations selected")
	}

	mini := ordered.New()
	copyRoot(mini, raw, "openapi")
	copyRoot(mini, raw, "info")
	copyRoot(mini, raw, "jsonSchemaDialect")
	copyRoot(mini, raw, "servers")
	copyRoot(mini, raw, "security")
	copyRoot(mini, raw, "tags")
	copyRoot(mini, raw, "externalDocs")

	paths, ok := asMap(raw["paths"])
	if !ok {
		return nil, fmt.Errorf("OpenAPI document has no paths object")
	}

	miniPaths := ordered.New()
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

		var target *ordered.Map
		if existing, ok := miniPaths.Get(op.Path); ok {
			target, _ = existing.(*ordered.Map)
		}
		if target == nil {
			target = ordered.New()
			miniPaths.Set(op.Path, target)
		}
		for _, field := range []string{"summary", "description", "servers", "parameters"} {
			if value, exists := pathItem[field]; exists {
				target.Set(field, value)
			}
		}
		target.Set(method, operation)
	}
	mini.Set("paths", miniPaths)

	filterTags(mini, raw, selected)
	if err := includeReachableComponents(mini, raw); err != nil {
		return nil, err
	}
	return mini, nil
}

func copyRoot(dst *ordered.Map, src map[string]any, key string) {
	if value, ok := src[key]; ok {
		dst.Set(key, value)
	}
}

func filterTags(mini *ordered.Map, raw map[string]any, selected []catalog.Operation) {
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
		mini.Set("tags", tags)
	}
}

func includeReachableComponents(mini *ordered.Map, raw map[string]any) error {
	rawComponents, ok := asMap(raw["components"])
	if !ok {
		return nil
	}
	miniComponents := ordered.New()
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

		var targetSection *ordered.Map
		if existing, ok := miniComponents.Get(section); ok {
			targetSection, _ = existing.(*ordered.Map)
		}
		if targetSection == nil {
			targetSection = ordered.New()
			miniComponents.Set(section, targetSection)
		}
		targetSection.Set(name, value)
		queue = append(queue, refsIn(value)...)
	}

	if miniComponents.Len() > 0 {
		mini.Set("components", orderComponentSections(miniComponents))
	}
	return nil
}

func includeSecuritySchemes(miniComponents *ordered.Map, rawComponents map[string]any, mini *ordered.Map) error {
	names := securitySchemeNames(mini)
	if len(names) == 0 {
		return nil
	}
	rawSchemes, ok := asMap(rawComponents["securitySchemes"])
	if !ok {
		return nil
	}
	target := ordered.New()
	for name := range names {
		value, ok := rawSchemes[name]
		if !ok {
			return fmt.Errorf("unresolved security scheme: %s", name)
		}
		target.Set(name, value)
	}
	if target.Len() > 0 {
		miniComponents.Set("securitySchemes", target)
	}
	return nil
}

func securitySchemeNames(value any) map[string]bool {
	names := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case *ordered.Map:
			if security, ok := typed.Values["security"]; ok {
				collectSecurityNames(names, security)
			}
			for _, key := range typed.Keys {
				if key == "security" {
					continue
				}
				walk(typed.Values[key])
			}
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
		case *ordered.Map:
			if ref, ok := typed.Values["$ref"].(string); ok {
				refs = append(refs, ref)
			}
			for _, key := range typed.Keys {
				walk(typed.Values[key])
			}
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

func orderComponentSections(components *ordered.Map) *ordered.Map {
	result := ordered.New()
	preferred := []string{
		"schemas",
		"responses",
		"parameters",
		"examples",
		"requestBodies",
		"headers",
		"securitySchemes",
		"links",
		"callbacks",
	}
	seen := map[string]bool{}
	for _, key := range preferred {
		if value, ok := components.Get(key); ok {
			result.Set(key, value)
			seen[key] = true
		}
	}
	keys := make([]string, 0)
	for _, key := range components.Keys {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		result.Set(key, components.Values[key])
	}
	return result
}
