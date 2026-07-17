package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
)

const (
	impressumTargetURL = "https://legal.example.com/impressum"
	privacyTargetURL   = "https://legal.example.com/privacy"
)

func newImpressumRouter(t *testing.T) http.Handler {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.ImpressumURL = impressumTargetURL
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return api.NewRouter(cfg, nil).SetupRoutes()
}

func newPrivacyRouter(t *testing.T) http.Handler {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.PrivacyURL = privacyTargetURL
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

func TestPrivacyPageServedAtStaticPaths(t *testing.T) {
	handler := api.NewRouter(config.DefaultConfig(), nil).SetupRoutes()
	for _, requestPath := range []string{"/privacy", "/privacy/", "/privacy.html"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, exampleBaseURL+requestPath, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: "+statusWantFmt, requestPath, rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get(routerContentTypeKey); !strings.HasPrefix(got, routerContentTypeHTML) {
			t.Fatalf("%s: "+routerContentTypeWantFmt, requestPath, got, routerContentTypeHTML)
		}
		if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
			t.Fatalf("%s cache-control = %q, want no-store", requestPath, got)
		}
	}
}

func TestPrivacyRedirectsToOperatorNoticeWhenConfigured(t *testing.T) {
	handler := newPrivacyRouter(t)
	for _, requestPath := range []string{
		"/privacy",
		"/privacy/",
		"/privacy.html",
		"/privacy.html/",
		"/alias/%2e%2e/privacy",
		"/alias/%2e%2e/privacy.html/",
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(method, exampleBaseURL+requestPath, nil))
			if rec.Code != http.StatusFound {
				t.Fatalf("%s %s: "+statusWantFmt, method, requestPath, rec.Code, http.StatusFound)
			}
			if got := rec.Header().Get("Location"); got != privacyTargetURL {
				t.Fatalf("%s %s location = %q, want %q", method, requestPath, got, privacyTargetURL)
			}
			if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
				t.Fatalf("%s %s cache-control = %q, want no-store", method, requestPath, got)
			}
		}
	}
}

func TestPrivacyRedirectDoesNotCaptureNestedPaths(t *testing.T) {
	handler := newPrivacyRouter(t)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, exampleBaseURL+"/privacy/not-a-page", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusNotFound)
	}
}
