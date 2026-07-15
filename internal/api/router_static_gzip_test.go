package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestAcceptsGzip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		header string
		want   bool
	}{
		{header: "gzip", want: true},
		{header: "br, GZip", want: true},
		{header: "gzip; q=0.5", want: true},
		{header: "br, *;q=0.2", want: true},
		{header: "gzip;q=0", want: false},
		{header: "gzip;q=0, *;q=1", want: false},
		{header: "gzip;q=invalid", want: false},
		{header: "x-gzip", want: false},
		{header: "br", want: false},
		{header: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			t.Parallel()
			if got := acceptsGzip(tt.header); got != tt.want {
				t.Fatalf("acceptsGzip(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestStaticAssetHandlerGzipSemantics(t *testing.T) {
	t.Parallel()
	payload := bytes.Repeat([]byte("openbyte-static-asset\n"), 128)
	modTime := time.Unix(1_700_000_000, 0)
	h := newStaticAllowlistHandler(http.FS(fstest.MapFS{
		"openbyte.js": &fstest.MapFile{Data: payload, ModTime: modTime},
	}))

	get := serveStaticRequest(h, http.MethodGet, "/openbyte.js", "br, gzip", "")
	if get.Code != http.StatusOK {
		t.Fatalf("gzip GET status = %d, want %d", get.Code, http.StatusOK)
	}
	if got := get.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("gzip GET Content-Encoding = %q, want gzip", got)
	}
	if got := get.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("gzip GET Vary = %q, want Accept-Encoding", got)
	}
	if got := get.Header().Get("Content-Length"); got != strconv.Itoa(get.Body.Len()) {
		t.Fatalf("gzip GET Content-Length = %q, body length %d", got, get.Body.Len())
	}
	if got := gunzipBody(t, get.Body.Bytes()); !bytes.Equal(got, payload) {
		t.Fatal("gzip GET body does not round-trip to the source asset")
	}

	head := serveStaticRequest(h, http.MethodHead, "/openbyte.js", "gzip", "")
	if head.Code != http.StatusOK {
		t.Fatalf("gzip HEAD status = %d, want %d", head.Code, http.StatusOK)
	}
	if head.Body.Len() != 0 {
		t.Fatalf("gzip HEAD body length = %d, want 0", head.Body.Len())
	}
	if got, want := head.Header().Get("Content-Length"), get.Header().Get("Content-Length"); got != want {
		t.Fatalf("gzip HEAD Content-Length = %q, want GET length %q", got, want)
	}

	ranged := serveStaticRequest(h, http.MethodGet, "/openbyte.js", "gzip", "bytes=0-9")
	if ranged.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d, want %d", ranged.Code, http.StatusPartialContent)
	}
	if got := ranged.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("range Content-Encoding = %q, want identity", got)
	}
	if got := ranged.Body.Bytes(); !bytes.Equal(got, payload[:10]) {
		t.Fatalf("range body = %q, want %q", got, payload[:10])
	}
	if got, want := ranged.Header().Get("Content-Range"), "bytes 0-9/"+strconv.Itoa(len(payload)); got != want {
		t.Fatalf("Content-Range = %q, want %q", got, want)
	}

	identity := serveStaticRequest(h, http.MethodGet, "/openbyte.js", "gzip;q=0", "")
	if got := identity.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("q=0 Content-Encoding = %q, want identity", got)
	}
	if got := identity.Body.Bytes(); !bytes.Equal(got, payload) {
		t.Fatal("q=0 response body differs from the source asset")
	}
	multiValueReq := httptest.NewRequest(http.MethodGet, "/openbyte.js", nil)
	multiValueReq.Header.Add("Accept-Encoding", "br")
	multiValueReq.Header.Add("Accept-Encoding", "gzip")
	multiValue := httptest.NewRecorder()
	h.ServeHTTP(multiValue, multiValueReq)
	if got := multiValue.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("multi-value Content-Encoding = %q, want gzip", got)
	}

	conditionalReq := httptest.NewRequest(http.MethodGet, "/openbyte.js", nil)
	conditionalReq.Header.Set("Accept-Encoding", "gzip")
	conditionalReq.Header.Set("If-None-Match", `"asset-v1"`)
	conditional := httptest.NewRecorder()
	conditional.Header().Set("ETag", `"asset-v1"`)
	h.ServeHTTP(conditional, conditionalReq)
	if conditional.Code != http.StatusNotModified {
		t.Fatalf("conditional status = %d, want %d", conditional.Code, http.StatusNotModified)
	}
	if got := conditional.Header().Get("ETag"); got != `"asset-v1"` {
		t.Fatalf("conditional ETag = %q, want asset-v1", got)
	}
	if got := conditional.Header().Get("Content-Length"); got != "" {
		t.Fatalf("conditional Content-Length = %q, want empty", got)
	}

	failedPreconditionReq := httptest.NewRequest(http.MethodGet, "/openbyte.js", nil)
	failedPreconditionReq.Header.Set("Accept-Encoding", "gzip")
	failedPreconditionReq.Header.Set("If-Match", `"missing"`)
	failedPrecondition := httptest.NewRecorder()
	h.ServeHTTP(failedPrecondition, failedPreconditionReq)
	if failedPrecondition.Code != http.StatusPreconditionFailed {
		t.Fatalf("If-Match status = %d, want %d", failedPrecondition.Code, http.StatusPreconditionFailed)
	}
	if got := failedPrecondition.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("failed precondition Content-Encoding = %q, want empty", got)
	}
	if got := failedPrecondition.Header().Get("Content-Length"); got != "" {
		t.Fatalf("failed precondition Content-Length = %q, want empty", got)
	}

	modifiedReq := httptest.NewRequest(http.MethodGet, "/openbyte.js", nil)
	modifiedReq.Header.Set("Accept-Encoding", "gzip")
	modifiedReq.Header.Set("If-Modified-Since", get.Header().Get("Last-Modified"))
	modified := httptest.NewRecorder()
	h.ServeHTTP(modified, modifiedReq)
	if modified.Code != http.StatusNotModified {
		t.Fatalf("If-Modified-Since status = %d, want %d", modified.Code, http.StatusNotModified)
	}
}

func TestStaticAssetHandlerSkipsFontGzip(t *testing.T) {
	t.Parallel()
	payload := []byte("already-compressed-font")
	const fontName = "fonts/dm-sans-latin.woff2"
	h := newStaticAllowlistHandler(http.FS(fstest.MapFS{
		fontName: &fstest.MapFile{Data: payload},
	}))

	rec := serveStaticRequest(h, http.MethodGet, "/"+fontName, "gzip", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("font status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("font Content-Encoding = %q, want identity", got)
	}
	if got := rec.Header().Get("Vary"); strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("font Vary = %q, should not vary by Accept-Encoding", got)
	}
	if got := rec.Body.Bytes(); !bytes.Equal(got, payload) {
		t.Fatal("font response body differs from source")
	}
}

func TestStaticAssetHandlerRefreshesWebRootGzip(t *testing.T) {
	t.Parallel()
	webRoot := t.TempDir()
	assetPath := filepath.Join(webRoot, "openbyte.js")
	first := []byte("first static asset version")
	second := bytes.Repeat([]byte("x"), len(first))
	third := []byte("third static asset version with a different size")
	if err := os.WriteFile(assetPath, first, 0o644); err != nil {
		t.Fatalf("write first asset: %v", err)
	}

	h := newStaticAllowlistHandler(http.Dir(webRoot)).(*staticAssetHandler)
	firstRec := serveStaticRequest(h, http.MethodGet, "/openbyte.js", "gzip", "")
	if got := gunzipBody(t, firstRec.Body.Bytes()); !bytes.Equal(got, first) {
		t.Fatal("first WEB_ROOT response differs from source")
	}

	if err := os.WriteFile(assetPath, second, 0o644); err != nil {
		t.Fatalf("write second asset: %v", err)
	}
	newModTime := time.Now().Add(time.Second)
	if err := os.Chtimes(assetPath, newModTime, newModTime); err != nil {
		t.Fatalf("update asset modtime: %v", err)
	}
	secondRec := serveStaticRequest(h, http.MethodGet, "/openbyte.js", "gzip", "")
	if got := gunzipBody(t, secondRec.Body.Bytes()); !bytes.Equal(got, second) {
		t.Fatal("updated WEB_ROOT response was not recompressed")
	}
	if err := os.WriteFile(assetPath, third, 0o644); err != nil {
		t.Fatalf("write third asset: %v", err)
	}
	if err := os.Chtimes(assetPath, newModTime, newModTime); err != nil {
		t.Fatalf("preserve asset modtime: %v", err)
	}
	thirdRec := serveStaticRequest(h, http.MethodGet, "/openbyte.js", "gzip", "")
	if got := gunzipBody(t, thirdRec.Body.Bytes()); !bytes.Equal(got, third) {
		t.Fatal("resized WEB_ROOT response was not recompressed")
	}

	h.gzipMu.Lock()
	defer h.gzipMu.Unlock()
	if got := len(h.gzipCache); got != 1 {
		t.Fatalf("gzip cache entries = %d, want one current version", got)
	}
	if got := h.gzipCache["openbyte.js"].size; got != int64(len(third)) {
		t.Fatalf("cached asset size = %d, want %d", got, len(third))
	}
}

func serveStaticRequest(h http.Handler, method, target, acceptEncoding, byteRange string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func gunzipBody(t *testing.T, body []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("open gzip response: %v", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip response: %v", err)
	}
	return data
}
