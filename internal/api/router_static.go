package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/saveenergy/openbyte/web"
)

type staticGzipAsset struct {
	size    int64
	modTime time.Time
	data    []byte
}

type staticAssetHandler struct {
	webFS     http.FileSystem
	gzipMu    sync.Mutex
	gzipCache map[string]staticGzipAsset
}

func newStaticAllowlistHandler(webFS http.FileSystem) http.Handler {
	return &staticAssetHandler{
		webFS:     webFS,
		gzipCache: make(map[string]staticGzipAsset),
	}
}

var embeddedStaticAssets = loadEmbeddedStaticAssets()

func loadEmbeddedStaticAssets() map[string]bool {
	assets := make(map[string]bool)
	err := fs.WalkDir(web.Assets, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			assets[name] = true
		}
		return nil
	})
	if err != nil {
		panic("read embedded web assets: " + err.Error())
	}
	return assets
}

func (h *staticAssetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	name := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if name == "." || name == "/" {
		name = "index.html"
	}
	switch name {
	case "results":
		name += ".html"
	}
	if strings.Contains(name, "..") || !isAllowedStaticAsset(name) {
		http.NotFound(w, r)
		return
	}
	f, err := h.webFS.Open(name)
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

	gzipCandidate := !strings.HasPrefix(name, "fonts/")
	if gzipCandidate {
		w.Header().Add("Vary", "Accept-Encoding")
	}
	acceptEncoding := strings.Join(r.Header.Values("Accept-Encoding"), ",")
	if gzipCandidate && r.Header.Get("Range") == "" && acceptsGzip(acceptEncoding) {
		compressed, compressErr := h.gzipAsset(f, name, stat.Size(), stat.ModTime())
		if compressErr == nil {
			http.ServeContent(
				&staticGzipResponseWriter{ResponseWriter: w},
				r,
				name,
				stat.ModTime(),
				bytes.NewReader(compressed),
			)
			return
		}
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	http.ServeContent(w, r, name, stat.ModTime(), f)
}

type staticGzipResponseWriter struct {
	http.ResponseWriter
}

func (w *staticGzipResponseWriter) WriteHeader(status int) {
	if status == http.StatusOK {
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.ResponseWriter.WriteHeader(status)
}

func (h *staticAssetHandler) gzipAsset(f http.File, name string, size int64, modTime time.Time) ([]byte, error) {
	h.gzipMu.Lock()
	defer h.gzipMu.Unlock()
	if cached, ok := h.gzipCache[name]; ok && cached.size == size && cached.modTime.Equal(modTime) {
		return cached.data, nil
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, copyErr := io.Copy(gz, f)
	closeErr := gz.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	compressed := buf.Bytes()
	h.gzipCache[name] = staticGzipAsset{size: size, modTime: modTime, data: compressed}
	return compressed, nil
}

func acceptsGzip(header string) bool {
	gzipQuality, gzipFound := encodingQuality(header, "gzip")
	if gzipFound {
		return gzipQuality > 0
	}
	wildcardQuality, wildcardFound := encodingQuality(header, "*")
	return wildcardFound && wildcardQuality > 0
}

func encodingQuality(header, wanted string) (float64, bool) {
	var quality float64
	found := false
	for part := range strings.SplitSeq(header, ",") {
		fields := strings.Split(part, ";")
		if !strings.EqualFold(strings.TrimSpace(fields[0]), wanted) {
			continue
		}
		quality = 1
		found = true
		for _, param := range fields[1:] {
			key, value, ok := strings.Cut(param, "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(key), "q") {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil || parsed < 0 || parsed > 1 {
				quality = 0
			} else {
				quality = parsed
			}
		}
	}
	return quality, found
}

func isAllowedStaticAsset(name string) bool {
	return embeddedStaticAssets[name]
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
