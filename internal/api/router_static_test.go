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

func TestIsAllowedStaticAsset_matchesEmbeddedFiles(t *testing.T) {
	t.Parallel()

	for _, asset := range []string{
		"openbyte.js",
		"i18n.js",
		"locale-en.js",
		"locale-de.js",
		"presentation.js",
	} {
		if !isAllowedStaticAsset(asset) {
			t.Fatalf("expected embedded module %q allowed", asset)
		}
	}
	if !isAllowedStaticAsset("fonts/dm-sans-latin.woff2") {
		t.Fatal("expected embedded font allowed")
	}
	if isAllowedStaticAsset("fonts/unbundled.woff2") {
		t.Fatal("expected unbundled font rejected")
	}
	if isAllowedStaticAsset("embed.go") {
		t.Fatal("expected source file rejected")
	}
}
