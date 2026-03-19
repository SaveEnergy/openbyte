package client

import (
	"fmt"
	"strconv"
	"strings"
)

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatNumber(n int64) string {
	s := strconv.FormatInt(n, 10)
	start := 0
	var result strings.Builder
	if len(s) > 0 && s[0] == '-' {
		result.WriteByte('-')
		start = 1
	}
	digits := s[start:]
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(r)
	}
	return result.String()
}
