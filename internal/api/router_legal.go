package api

import (
	"net/http"
	"path"
)

// The Impressum (legal notice) is operator-specific content that openByte
// cannot author. When IMPRESSUM_URL is configured, /impressum redirects to the
// operator's document and /branding.css unhides the footer link that points
// here; unconfigured deployments keep the route as a 404 and the link hidden.
const impressumPath = "/impressum"
const privacyPath = "/privacy"

func (r *Router) serveImpressumRedirect(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(headerCacheControl, valueNoStore)
	if r.impressumURL == "" {
		http.NotFound(w, req)
		return
	}
	http.Redirect(w, req, r.impressumURL, http.StatusFound)
}

// PRIVACY_URL lets each deployment provide the controller-specific Article 13
// notice that generic self-hosted software cannot author. Without it, /privacy
// serves openByte's bundled technical data-handling summary.
func (r *Router) servePrivacy(w http.ResponseWriter, req *http.Request, fallback http.Handler) {
	w.Header().Set(headerCacheControl, valueNoStore)
	if !isPrivacyPath(req.URL.Path) {
		http.NotFound(w, req)
		return
	}
	if r.privacyURL != "" {
		http.Redirect(w, req, r.privacyURL, http.StatusFound)
		return
	}
	fallback.ServeHTTP(w, req)
}

func isPrivacyPath(requestPath string) bool {
	cleanPath := path.Clean(requestPath)
	return cleanPath == privacyPath || cleanPath == privacyPath+".html"
}
