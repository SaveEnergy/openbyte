package e2e

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFrontendLoads(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.baseURL + "/")
	if err != nil {
		t.Fatalf("Frontend request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Frontend status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read frontend: %v", err)
	}

	html := string(body)

	checks := []string{
		"<title>openByte",
		"openByte",
		"openbyte.js",
		"style.css",
	}

	for _, check := range checks {
		if !strings.Contains(html, check) {
			t.Errorf("Frontend missing: %s", check)
		}
	}
}

func TestStaticFiles(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	files := []string{
		openbyteJSPath,
		"/style.css",
	}

	for _, file := range files {
		resp, err := http.Get(ts.baseURL + file)
		if err != nil {
			t.Errorf("Failed to load %s: %v", file, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			if _, drainErr := io.Copy(io.Discard, resp.Body); drainErr != nil {
				t.Errorf("failed to drain %s response body: %v", file, drainErr)
			}
			t.Errorf("%s status = %d, want %d", file, resp.StatusCode, http.StatusOK)
		}

		contentType := resp.Header.Get("Content-Type")
		if file == openbyteJSPath && !strings.Contains(contentType, "javascript") {
			t.Errorf("%s content-type = %s, want javascript", file, contentType)
		}
		if file == "/style.css" && !strings.Contains(contentType, "css") {
			t.Errorf("%s content-type = %s, want css", file, contentType)
		}
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close %s response body: %v", file, err)
		}
	}
}

func TestJavaScriptFunctions(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.baseURL + openbyteJSPath)
	if err != nil {
		t.Fatalf("Failed to load openbyte.js: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("openbyte.js status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read openbyte.js: %v", err)
	}

	js := string(body)

	requiredFunctions := []string{
		"function startTest",
		"function runTest",
		"function updateSpeed",
		"function showResults",
		"function resetToIdle",
		"function showError",
	}

	for _, fn := range requiredFunctions {
		if !strings.Contains(js, fn) {
			t.Errorf("Missing required function: %s", fn)
		}
	}

	requiredVars := []string{
		"apiBase",
		"const state",
		"const elements",
	}

	for _, v := range requiredVars {
		if !strings.Contains(js, v) {
			t.Errorf("Missing required variable: %s", v)
		}
	}

	openBraces := strings.Count(js, "{")
	closeBraces := strings.Count(js, "}")
	if openBraces != closeBraces {
		t.Errorf("Unbalanced braces: %d open, %d close", openBraces, closeBraces)
	}
}
