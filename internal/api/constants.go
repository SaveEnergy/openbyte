package api

// Error and path literals for S1192.
const (
	errNotFound          = "not found"
	errRateLimitExceeded = "rate limit exceeded"
	errOriginNotAllowed  = "origin not allowed"
	apiV1Prefix          = "/api/v1"

	headerCacheControl = "Cache-Control"
	valueNoStore       = "no-store"
	headerRetryAfter   = "Retry-After"
	retryAfterSec      = "60"
	resultsHTML        = "results.html"
)
