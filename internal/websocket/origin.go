package websocket

import (
	"net/url"
	"strings"

	"github.com/saveenergy/openbyte/pkg/types"
)

func (s *Server) isAllowedOrigin(origin, host string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allowedOrigins := s.allowedOrigins

	if origin == "" {
		return allowEmptyOrigin(allowedOrigins)
	}

	if len(allowedOrigins) == 0 {
		return sameOrigin(origin, host)
	}

	originHostValue := types.OriginHost(origin)
	for _, allowed := range allowedOrigins {
		if matchesAllowedOrigin(allowed, origin, originHostValue) {
			return true
		}
	}
	return false
}

func allowEmptyOrigin(allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return true
	}
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true
		}
	}
	return false
}

func matchesAllowedOrigin(allowed, origin, originHostValue string) bool {
	if allowed == "" {
		return false
	}
	if allowed == "*" || allowed == origin || strings.EqualFold(allowed, origin) {
		return true
	}
	if after, ok := strings.CutPrefix(allowed, "*."); ok {
		suffix := after
		return originHostValue != "" && (originHostValue == suffix || strings.HasSuffix(originHostValue, "."+suffix))
	}
	allowedHost := types.OriginHost(allowed)
	return allowedHost != "" && originHostValue != "" && strings.EqualFold(allowedHost, originHostValue)
}

func sameOrigin(origin, host string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originH := types.StripHostPort(parsed.Host)
	requestH := types.StripHostPort(host)
	return strings.EqualFold(originH, requestH)
}
