package specio

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"sigs.k8s.io/yaml"
)

type Loaded struct {
	Doc *openapi3.T
	Raw map[string]any
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func Load(source string, stdin io.Reader) (*Loaded, error) {
	var data []byte
	var err error
	if source == "-" {
		data, err = io.ReadAll(stdin)
	} else if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		data, err = readURL(source)
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
		return nil, fmt.Errorf("not a valid OpenAPI document: %s", summarizeParseError(err))
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

// summarizeParseError trims internal Go type names and multi-error chains down
// to the most useful first line for a user. kin-openapi's combined json/yaml
// error mentions internal generated types (openapi3.TBis etc.) that confuse
// readers; we keep only the first error path.
func summarizeParseError(err error) string {
	msg := err.Error()
	if idx := strings.Index(msg, ", yaml error:"); idx >= 0 {
		msg = msg[:idx]
	}
	if idx := strings.Index(msg, " of type openapi3."); idx >= 0 {
		msg = msg[:idx]
	}
	msg = strings.TrimPrefix(msg, "failed to unmarshal data: ")
	if idx := strings.Index(msg, "\n"); idx >= 0 {
		msg = msg[:idx]
	}
	return strings.TrimSpace(msg)
}

func readURL(source string) ([]byte, error) {
	resp, err := httpClient.Get(source)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}
