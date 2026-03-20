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
		"download.html":              true,
		resultsHTML:                  true,
		"skill.html":                 true,
		"openbyte.js":                true,
		"state.js":                   true,
		"utils.js":                   true,
		"network.js":                 true,
		"network-helpers.js":         true,
		"network-health.js":          true,
		"speedtest.js":               true,
		"speedtest-http.js":          true,
		"speedtest-http-download.js": true,
		"speedtest-http-shared.js":   true,
		"speedtest-http-upload.js":   true,
		"speedtest-latency.js":       true,
		"warmup.js":                  true,
		"diagnostics.js":             true,
		"settings.js":                true,
		"ui.js":                      true,
		"download.js":                true,
		"download-platform.js":       true,
		"download-github.js":         true,
		"results.js":                 true,
		"skill.js":                   true,
		"base.css":                   true,
		"download.css":               true,
		"speed.css":                  true,
		"modal.css":                  true,
		"skill.css":                  true,
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
		case "download", "results", "skill":
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

func isAllowedStaticAsset(name string, allowed map[string]bool) bool {
	if allowed[name] {
		return true
	}
	if strings.HasPrefix(name, "fonts/") {
		return strings.HasSuffix(name, ".woff2") || strings.HasSuffix(name, ".woff")
	}
	return false
}

func staticCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
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
