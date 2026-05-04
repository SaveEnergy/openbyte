package api

import (
	"compress/gzip"
	"net/http"
	"path"
	"strings"
)

func newStaticAllowlistHandler(webFS http.FileSystem) http.Handler {
	allowed := map[string]bool{
		"index.html":                 true,
		"api.html":                   true,
		resultsHTML:                  true,
		"openbyte.js":                true,
		"state.js":                   true,
		"utils.js":                   true,
		"network.js":                 true,
		"network-probes.js":          true,
		"network-helpers.js":         true,
		"network-health.js":          true,
		"speedtest-orchestrator.js":  true,
		"speedtest-adaptive.js":      true,
		"speedtest-worker.js":        true,
		"speedtest.js":               true,
		"speedtest-http.js":          true,
		"speedtest-http-download.js": true,
		"speedtest-http-shared.js":   true,
		"speedtest-http-upload.js":   true,
		"speedtest-latency.js":       true,
		"warmup.js":                  true,
		"diagnostics.js":             true,
		"events.js":                  true,
		"ui.js":                      true,
		"results.js":                 true,
		"api.css":                    true,
		"base.css":                   true,
		"speed.css":                  true,
		"toast.css":                  true,
		"motion.css":                 true,
		"favicon.svg":                true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		name := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if name == "." || name == "/" {
			name = "index.html"
		}
		switch name {
		case "api", "results":
			name += ".html"
		}
		if strings.Contains(name, "..") || !isAllowedStaticAsset(name, allowed) {
			http.NotFound(w, r)
			return
		}
		f, err := webFS.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, name, stat.ModTime(), f)
	})
}

const (
	staticFontsDirPrefix    = "fonts/"
	staticFontsDirPrefixLen = len(staticFontsDirPrefix)
	staticWoff2Suffix       = ".woff2"
	staticWoff2SuffixLen    = len(staticWoff2Suffix)
	staticWoffSuffix        = ".woff"
	staticWoffSuffixLen     = len(staticWoffSuffix)
)

func isAllowedStaticAsset(name string, allowed map[string]bool) bool {
	if allowed[name] {
		return true
	}
	if len(name) >= staticFontsDirPrefixLen && name[:staticFontsDirPrefixLen] == staticFontsDirPrefix {
		return staticFontAssetSuffixOK(name)
	}
	return false
}

func staticFontAssetSuffixOK(name string) bool {
	n := len(name)
	if n >= staticWoff2SuffixLen && name[n-staticWoff2SuffixLen:] == staticWoff2Suffix {
		return true
	}
	return n >= staticWoffSuffixLen && name[n-staticWoffSuffixLen:] == staticWoffSuffix
}

func staticPathIsRootOrHTML(path string) bool {
	if path == "/" {
		return true
	}
	n := len(path)
	return n >= 5 && path[n-5:] == ".html"
}

func staticCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			if staticPathIsRootOrHTML(r.URL.Path) {
				w.Header().Set(headerCacheControl, valueNoStore)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// gzipMiddleware compresses static responses when client accepts gzip.
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gzW := gzip.NewWriter(w)
		defer gzW.Close()
		w.Header().Set("Content-Encoding", "gzip")
		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, Writer: gzW}, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer *gzip.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	return g.Writer.Write(b)
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	g.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeader(code)
}
