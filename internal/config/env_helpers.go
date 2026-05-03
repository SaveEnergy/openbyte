package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	envTrue  = "true"
	envOne   = "1"
	EnvDebug = "debug" // exported for cmd/server
	envFalse = "false"
	envZero  = "0"
)

func envBool(name string) bool {
	val := os.Getenv(name)
	return val == envTrue || val == envOne
}

func envCSV(name string) []string {
	raw := os.Getenv(name)
	if raw == "" {
		return nil
	}
	entries := strings.Split(raw, ",")
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		value := strings.TrimSpace(entry)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func parsePositiveIntEnv(name string) (int, bool, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, false, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, true, fmt.Errorf("invalid %s %q: must be a positive integer", name, raw)
	}
	return v, true, nil
}
