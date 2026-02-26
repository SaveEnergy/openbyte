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

	// Modular frontend: index uses ES module entry + CSS partials (no style.css monolith)
	checks := []string{
		"<title>openByte",
		"openByte",
		"openbyte.js",
		"base.css",
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

	files := []struct {
		path       string
		wantPartCT string
	}{{openbyteJSPath, "javascript"}, {"/base.css", "css"}}

	for _, f := range files {
		resp, err := http.Get(ts.baseURL + f.path)
		if err != nil {
			t.Errorf("Failed to load %s: %v", f.path, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			if _, drainErr := io.Copy(io.Discard, resp.Body); drainErr != nil {
				t.Errorf("failed to drain %s response body: %v", f.path, drainErr)
			}
			t.Errorf("%s status = %d, want %d", f.path, resp.StatusCode, http.StatusOK)
		}

		contentType := resp.Header.Get("Content-Type")
		if f.wantPartCT != "" && !strings.Contains(contentType, f.wantPartCT) {
			t.Errorf("%s content-type = %s, want %s", f.path, contentType, f.wantPartCT)
		}
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close %s response body: %v", f.path, err)
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

	// Modular frontend: ES module entry imports state/ui; orchestration lives in openbyte.js
	required := []string{
		"import",
		"startTest",
		"resetToIdle",
		"state.js",
	}

	for _, s := range required {
		if !strings.Contains(js, s) {
			t.Errorf("openbyte.js missing expected: %s", s)
		}
	}

	openBraces := strings.Count(js, "{")
	closeBraces := strings.Count(js, "}")
	if openBraces != closeBraces {
		t.Errorf("Unbalanced braces: %d open, %d close", openBraces, closeBraces)
	}
}
