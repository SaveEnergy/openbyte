package api_test

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestOpenAPIRouteContract(t *testing.T) {
	got := loadOpenAPIRoutes(t)

	expected := map[string]struct{}{
		"GET /health":              {},
		"GET /api/v1/ping":         {},
		"GET /api/v1/download":     {},
		"POST /api/v1/upload":      {},
		"POST /api/v1/results":     {},
		"GET /api/v1/results/{id}": {},
	}

	missing := diff(expected, got)
	extra := diff(got, expected)

	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("OpenAPI route contract mismatch\nmissing in spec: %v\nextra in spec: %v", missing, extra)
	}
}

func loadOpenAPIRoutes(t *testing.T) map[string]struct{} {
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

	routes := make(map[string]struct{})
	var currentPath string
	inPaths := false
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "paths:" {
			inPaths = true
			continue
		}
		if !inPaths {
			continue
		}
		if line != "" && line[0] != ' ' {
			break
		}
		if strings.HasPrefix(line, "  /") && strings.HasSuffix(line, ":") {
			currentPath = strings.TrimSuffix(strings.TrimSpace(line), ":")
			continue
		}
		if currentPath == "" || !strings.HasPrefix(line, "    ") {
			continue
		}
		method := strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(line), ":"))
		if isHTTPMethod(method) {
			routes[method+" "+currentPath] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan OpenAPI spec: %v", err)
	}
	if len(routes) == 0 {
		t.Fatal("OpenAPI spec has no paths")
	}
	return routes
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
