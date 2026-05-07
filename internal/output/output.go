package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/atotto/clipboard"
	"github.com/korECM/openapi-extract/internal/ordered"
	yamlv3 "gopkg.in/yaml.v3"
)

func Marshal(value any, format string) ([]byte, error) {
	switch format {
	case "", "yaml", "yml":
		node, err := yamlNode(value)
		if err != nil {
			return nil, fmt.Errorf("failed to encode YAML: %w", err)
		}
		var b bytes.Buffer
		encoder := yamlv3.NewEncoder(&b)
		encoder.SetIndent(2)
		if err := encoder.Encode(node); err != nil {
			encoder.Close()
			return nil, fmt.Errorf("failed to encode YAML: %w", err)
		}
		if err := encoder.Close(); err != nil {
			return nil, fmt.Errorf("failed to encode YAML: %w", err)
		}
		return b.Bytes(), nil
	case "json":
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to encode JSON: %w", err)
		}
		return append(data, '\n'), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

func yamlNode(value any) (*yamlv3.Node, error) {
	switch typed := value.(type) {
	case *ordered.Map:
		node := &yamlv3.Node{Kind: yamlv3.MappingNode}
		for _, key := range typed.Keys {
			child, err := yamlNode(typed.Values[key])
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, scalarNode(key), child)
		}
		return node, nil
	case map[string]any:
		node := &yamlv3.Node{Kind: yamlv3.MappingNode}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child, err := yamlNode(typed[key])
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, scalarNode(key), child)
		}
		return node, nil
	case []any:
		node := &yamlv3.Node{Kind: yamlv3.SequenceNode}
		for _, item := range typed {
			child, err := yamlNode(item)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, child)
		}
		return node, nil
	default:
		node := &yamlv3.Node{}
		if err := node.Encode(value); err != nil {
			return nil, err
		}
		return node, nil
	}
}

func scalarNode(value string) *yamlv3.Node {
	return &yamlv3.Node{Kind: yamlv3.ScalarNode, Tag: "!!str", Value: value}
}

func WriteFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}
	return nil
}

func Copy(data []byte) error {
	if err := clipboard.WriteAll(string(data)); err != nil {
		return fmt.Errorf("failed to copy output to clipboard: %w", err)
	}
	return nil
}
