package specio

import (
	"fmt"
	"io"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
	"sigs.k8s.io/yaml"
)

type Loaded struct {
	Doc *openapi3.T
	Raw map[string]any
}

func Load(source string, stdin io.Reader) (*Loaded, error) {
	var data []byte
	var err error
	if source == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(source)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAPI document: %w", err)
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI document: %w", err)
	}
	if doc.OpenAPI == "" {
		return nil, fmt.Errorf("unsupported OpenAPI version: empty")
	}
	if len(doc.OpenAPI) < 2 || doc.OpenAPI[:2] != "3." {
		return nil, fmt.Errorf("unsupported OpenAPI version: %s", doc.OpenAPI)
	}

	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize OpenAPI document: %w", err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(jsonData, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode OpenAPI document: %w", err)
	}

	return &Loaded{Doc: doc, Raw: raw}, nil
}
