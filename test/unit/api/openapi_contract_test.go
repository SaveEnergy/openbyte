package api_test

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type openAPISpec struct {
	Paths map[string]map[string]interface{} `yaml:"paths"`
}

func TestOpenAPIRouteContract(t *testing.T) {
	spec := loadOpenAPISpec(t)

	got := make(map[string]struct{})
	for path, operations := range spec.Paths {
		for method := range operations {
			upper := strings.ToUpper(strings.TrimSpace(method))
			if !isHTTPMethod(upper) {
				continue
			}
			got[upper+" "+path] = struct{}{}
		}
	}

	expected := map[string]struct{}{
		"GET /health":                          {},
		"GET /api/v1/ping":                     {},
		"GET /api/v1/download":                 {},
		"POST /api/v1/upload":                  {},
		"GET /api/v1/version":                  {},
		"GET /api/v1/servers":                  {},
		"POST /api/v1/stream/start":            {},
		"GET /api/v1/stream/{id}/status":       {},
		"GET /api/v1/stream/{id}/results":      {},
		"POST /api/v1/stream/{id}/cancel":      {},
		"POST /api/v1/stream/{id}/metrics":     {},
		"POST /api/v1/stream/{id}/complete":    {},
		"GET /api/v1/stream/{id}/stream":       {},
		"POST /api/v1/results":                 {},
		"GET /api/v1/results/{id}":             {},
		"GET /api/v1/registry/servers":         {},
		"POST /api/v1/registry/servers":        {},
		"GET /api/v1/registry/servers/{id}":    {},
		"PUT /api/v1/registry/servers/{id}":    {},
		"DELETE /api/v1/registry/servers/{id}": {},
		"GET /api/v1/registry/health":          {},
	}

	missing := diff(expected, got)
	extra := diff(got, expected)

	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("OpenAPI route contract mismatch\nmissing in spec: %v\nextra in spec: %v", missing, extra)
	}
}

func loadOpenAPISpec(t *testing.T) openAPISpec {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	specPath := filepath.Join(repoRoot, "api", "openapi.yaml")

	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read OpenAPI spec: %v", err)
	}

	var spec openAPISpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("unmarshal OpenAPI spec: %v", err)
	}
	if len(spec.Paths) == 0 {
		t.Fatal("OpenAPI spec has no paths")
	}
	return spec
}

func isHTTPMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "TRACE":
		return true
	default:
		return false
	}
}

func diff(left, right map[string]struct{}) []string {
	out := make([]string, 0)
	for key := range left {
		if _, ok := right[key]; !ok {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}
