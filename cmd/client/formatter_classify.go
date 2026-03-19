package client

import (
	"context"
	"errors"
	"net"
	"strings"
)

type messageCodeRule struct {
	code       string
	substrings []string
}

var messageCodeRules = []messageCodeRule{
	{code: "connection_refused", substrings: []string{"connection refused"}},
	{code: "server_unavailable", substrings: []string{"no such host"}},
	{code: "rate_limited", substrings: []string{"429", "rate limit"}},
	{code: "server_unavailable", substrings: []string{"503", "server at capacity"}},
	{code: "invalid_config", substrings: []string{"invalid", "must be"}},
	{code: "timeout", substrings: []string{"timeout", "deadline"}},
}

// classifyErrorCode maps an error to a machine-readable error code for JSON output.
func classifyErrorCode(err error) string {
	if err == nil {
		return "unknown"
	}

	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}

	var netErr *net.OpError
	if errors.As(err, &netErr) {
		if netErr.Op == "dial" {
			return "connection_refused"
		}
		if netErr.Timeout() {
			return "timeout"
		}
		return "network_error"
	}

	if code, ok := classifyMessageErrorCode(err.Error()); ok {
		return code
	}
	return "unknown"
}

func classifyMessageErrorCode(msg string) (string, bool) {
	for _, rule := range messageCodeRules {
		if containsAnySubstring(msg, rule.substrings) {
			return rule.code, true
		}
	}
	return "", false
}

func containsAnySubstring(msg string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}
