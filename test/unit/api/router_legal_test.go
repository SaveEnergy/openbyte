package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
)

const impressumTargetURL = "https://legal.example.com/impressum"

func newImpressumRouter(t *testing.T) http.Handler {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.ImpressumURL = impressumTargetURL
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return api.NewRouter(cfg, nil).SetupRoutes()
}

func TestImpressumRedirectsWhenConfigured(t *testing.T) {
	handler := newImpressumRouter(t)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, exampleBaseURL+"/impressum", nil))
		if rec.Code != http.StatusFound {
			t.Fatalf("%s: "+statusWantFmt, method, rec.Code, http.StatusFound)
		}
		if got := rec.Header().Get("Location"); got != impressumTargetURL {
			t.Fatalf("%s location = %q, want %q", method, got, impressumTargetURL)
		}
		if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
			t.Fatalf("%s cache-control = %q, want no-store", method, got)
		}
	}
}

func TestImpressumReturnsNotFoundWhenUnset(t *testing.T) {
	handler := api.NewRouter(config.DefaultConfig(), nil).SetupRoutes()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, exampleBaseURL+"/impressum", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
		t.Fatalf("cache-control = %q, want no-store", got)
	}
}

func TestBrandingCSSTogglesImpressumFooterLink(t *testing.T) {
	handler := newImpressumRouter(t)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, exampleBaseURL+"/branding.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusOK)
	}
	const want = ".footer-impressum { display: contents; }"
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("branding CSS missing %q:\n%s", want, rec.Body.String())
	}
}

func TestPrivacyPageServedAtCleanPath(t *testing.T) {
	handler := api.NewRouter(config.DefaultConfig(), nil).SetupRoutes()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, exampleBaseURL+"/privacy", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(routerContentTypeKey); !strings.HasPrefix(got, routerContentTypeHTML) {
		t.Fatalf(routerContentTypeWantFmt, got, routerContentTypeHTML)
	}
	if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
		t.Fatalf("cache-control = %q, want no-store", got)
	}
}
