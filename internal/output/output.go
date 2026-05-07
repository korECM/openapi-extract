package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/atotto/clipboard"
	"sigs.k8s.io/yaml"
)

func Marshal(value any, format string) ([]byte, error) {
	switch format {
	case "", "yaml", "yml":
		jsonData, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to encode JSON: %w", err)
		}
		data, err := yaml.JSONToYAML(jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to encode YAML: %w", err)
		}
		return data, nil
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
