package api

import (
	"strings"
	"testing"
)

func TestStaticPathIsRootOrHTML_matchesHasSuffix(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"/", "/index.html", "/foo.html", "/foo", "/foo.htm", "", "/a.html", "/path/x.html"} {
		want := p == "/" || strings.HasSuffix(p, ".html")
		if got := staticPathIsRootOrHTML(p); got != want {
			t.Fatalf("%q: got %v want %v", p, got, want)
		}
	}
}

func TestIsAllowedStaticAsset_fonts(t *testing.T) {
	t.Parallel()
	allowed := map[string]bool{"index.html": true}

	if !isAllowedStaticAsset("fonts/inter.woff2", allowed) {
		t.Fatal("expected fonts/*.woff2 allowed")
	}
	if !isAllowedStaticAsset("fonts/inter.woff", allowed) {
		t.Fatal("expected fonts/*.woff allowed")
	}
	if isAllowedStaticAsset("fonts/inter.ttf", allowed) {
		t.Fatal("expected fonts/*.ttf rejected")
	}
	if isAllowedStaticAsset("fonts/inter.woff2.backup", allowed) {
		t.Fatal("expected non-suffix path rejected")
	}
	if isAllowedStaticAsset("static/fonts/x.woff2", allowed) {
		t.Fatal("expected only fonts/ at root of name")
	}
}
