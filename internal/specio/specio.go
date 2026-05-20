package specio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"sigs.k8s.io/yaml"
)

type Loaded struct {
	Doc *openapi3.T
	Raw map[string]any
	// RawBytes is the size in bytes of the source document as it was read,
	// before normalization. Useful for reporting size reduction in the CLI.
	RawBytes int
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Options controls Load behavior. The zero value is the documented default:
// caching enabled, HTTP conditional fetch when revalidation metadata exists.
type Options struct {
	NoCache      bool
	RefreshCache bool
	// CacheLogger, when non-nil, receives one short status message per URL
	// fetch describing cache behavior (e.g. "hit (304)", "miss → 200 stored",
	// "bypassed"). Local file / stdin sources do not emit messages.
	CacheLogger func(msg string)
}

func Load(source string, stdin io.Reader) (*Loaded, error) {
	return LoadWithOptions(source, stdin, Options{})
}

func LoadWithOptions(source string, stdin io.Reader, opts Options) (*Loaded, error) {
	var data []byte
	var err error
	if source == "-" {
		data, err = io.ReadAll(stdin)
	} else if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		data, err = readURL(source, opts)
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

	return &Loaded{Doc: doc, Raw: raw, RawBytes: len(data)}, nil
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

func readURL(source string, opts Options) ([]byte, error) {
	log := opts.CacheLogger
	if log == nil {
		log = func(string) {}
	}
	if opts.NoCache {
		log("bypassed (--no-cache)")
		return fetchFresh(source, nil)
	}

	cachePath, metaPath, ok := cachePaths(source)
	cached, meta, hasCache := readCachedURL(cachePath, metaPath, ok)
	if opts.RefreshCache {
		hasCache, cached, meta = false, nil, cacheMeta{}
	}

	header := http.Header{}
	if hasCache {
		if meta.ETag != "" {
			header.Set("If-None-Match", meta.ETag)
		}
		if meta.LastModified != "" {
			header.Set("If-Modified-Since", meta.LastModified)
		}
	}

	req, err := http.NewRequest(http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if hasCache {
			log("stale-hit (network error)")
			return cached, nil
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified && hasCache {
		log("hit (304 Not Modified)")
		return cached, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if ok {
		writeCachedURL(cachePath, metaPath, body, cacheMeta{
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
			FetchedAt:    time.Now().UTC().Format(time.RFC3339),
		})
		if opts.RefreshCache {
			log(fmt.Sprintf("refresh → %s, stored (%d bytes)", resp.Status, len(body)))
		} else if hasCache {
			log(fmt.Sprintf("revalidated → %s, stored (%d bytes)", resp.Status, len(body)))
		} else {
			log(fmt.Sprintf("miss → %s, stored (%d bytes)", resp.Status, len(body)))
		}
	} else {
		log(fmt.Sprintf("miss → %s (cache unavailable)", resp.Status))
	}
	return body, nil
}

func fetchFresh(source string, header http.Header) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

type cacheMeta struct {
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	FetchedAt    string `json:"fetchedAt,omitempty"`
}

func cachePaths(source string) (string, string, bool) {
	dir, err := cacheDir()
	if err != nil {
		return "", "", false
	}
	sum := sha256.Sum256([]byte(source))
	key := hex.EncodeToString(sum[:])
	return filepath.Join(dir, key+".bin"), filepath.Join(dir, key+".meta.json"), true
}

func cacheDir() (string, error) {
	if env := os.Getenv("OPENAPI_EXTRACT_CACHE_DIR"); env != "" {
		if err := os.MkdirAll(env, 0o755); err != nil {
			return "", err
		}
		return env, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "openapi-extract")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func readCachedURL(cachePath, metaPath string, ok bool) ([]byte, cacheMeta, bool) {
	if !ok {
		return nil, cacheMeta{}, false
	}
	body, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, cacheMeta{}, false
	}
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return body, cacheMeta{}, true
	}
	var meta cacheMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return body, cacheMeta{}, true
	}
	return body, meta, true
}

func writeCachedURL(cachePath, metaPath string, body []byte, meta cacheMeta) {
	if err := os.WriteFile(cachePath, body, 0o644); err != nil {
		return
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return
	}
	_ = os.WriteFile(metaPath, metaBytes, 0o644)
}
