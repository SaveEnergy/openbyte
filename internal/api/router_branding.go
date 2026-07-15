package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/saveenergy/openbyte/internal/config"
)

const (
	brandingCSSContentType = "text/css; charset=utf-8"
	brandingCSSPath        = "/branding.css"
	brandingLogoPath       = "/branding/logo"
)

func renderBrandingCSS(palette config.BrandPalette, colorsConfigured, logoConfigured bool) []byte {
	if !colorsConfigured && !logoConfigured {
		return nil
	}

	var css strings.Builder
	if logoConfigured {
		css.WriteString(".brand-wordmark { display: none; }\n")
		css.WriteString(".brand-logo { display: block; }\n")
	}
	if colorsConfigured {
		css.WriteString(":root {\n")
		writeBrandTheme(&css, palette.Dark, palette.HasPrimary, palette.HasSecondary, "  ")
		css.WriteString("}\n")
		css.WriteString("@media (prefers-color-scheme: light) {\n")
		css.WriteString("  :root:not([data-theme=\"dark\"]) {\n")
		writeBrandTheme(&css, palette.Light, palette.HasPrimary, palette.HasSecondary, "    ")
		css.WriteString("  }\n}\n")
		css.WriteString(":root[data-theme=\"light\"] {\n")
		writeBrandTheme(&css, palette.Light, palette.HasPrimary, palette.HasSecondary, "  ")
		css.WriteString("}\n")
	}
	return []byte(css.String())
}

func writeBrandTheme(
	css *strings.Builder,
	theme config.BrandTheme,
	hasPrimary bool,
	hasSecondary bool,
	indent string,
) {
	if hasPrimary {
		writeCSSVariable(css, indent, "brand-primary", theme.Primary)
		writeCSSVariable(css, indent, "brand-secondary", theme.AccentSecondary)
		writeCSSVariable(css, indent, "on-brand", theme.OnBrand)
		writeCSSVariable(css, indent, "accent-primary", theme.Primary)
		writeCSSVariable(css, indent, "accent-secondary", theme.AccentSecondary)
		writeCSSVariable(css, indent, "accent-glow", theme.AccentGlow)
		writeCSSVariable(css, indent, "ambient-primary", theme.AmbientPrimary)
		writeCSSVariable(css, indent, "download-color", theme.Primary)
	}
	if hasSecondary {
		writeCSSVariable(css, indent, "ambient-secondary", theme.AmbientSecondary)
		writeCSSVariable(css, indent, "upload-color", theme.Secondary)
	}
}

func writeCSSVariable(css *strings.Builder, indent, name, value string) {
	fmt.Fprintf(css, "%s--%s: %s;\n", indent, name, value)
}

func (r *Router) serveBrandingCSS(w http.ResponseWriter, req *http.Request) {
	writeBrandingAsset(w, req, r.brandingCSS, brandingCSSContentType)
}

func (r *Router) serveBrandLogo(w http.ResponseWriter, req *http.Request) {
	if len(r.brandLogo.Data) == 0 || r.brandLogo.ContentType == "" {
		w.Header().Set(headerCacheControl, valueNoStore)
		http.NotFound(w, req)
		return
	}
	writeBrandingAsset(w, req, r.brandLogo.Data, r.brandLogo.ContentType)
}

func writeBrandingAsset(
	w http.ResponseWriter,
	req *http.Request,
	data []byte,
	contentType string,
) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set(headerCacheControl, valueNoStore)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	if req.Method == http.MethodHead || len(data) == 0 {
		return
	}
	if _, err := w.Write(data); err != nil {
		slog.Warn("branding: write asset", "path", req.URL.Path, "error", err)
	}
}
