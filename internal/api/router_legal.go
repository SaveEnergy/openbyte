package api

import "net/http"

// The Impressum (legal notice) is operator-specific content that openByte
// cannot author. When IMPRESSUM_URL is configured, /impressum redirects to the
// operator's document and /branding.css unhides the footer link that points
// here; unconfigured deployments keep the route as a 404 and the link hidden.
const impressumPath = "/impressum"

func (r *Router) serveImpressumRedirect(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(headerCacheControl, valueNoStore)
	if r.impressumURL == "" {
		http.NotFound(w, req)
		return
	}
	http.Redirect(w, req, r.impressumURL, http.StatusFound)
}
