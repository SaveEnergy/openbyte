package api

// forwardedProtoIsHTTPS reports whether s is an ASCII case-insensitive "https".
// It is used for X-Forwarded-Proto; values that are not exactly five ASCII bytes
// are rejected (including Unicode lookalikes).
func forwardedProtoIsHTTPS(s string) bool {
	if len(s) != 5 {
		return false
	}
	return (s[0]|32) == 'h' &&
		(s[1]|32) == 't' &&
		(s[2]|32) == 't' &&
		(s[3]|32) == 'p' &&
		(s[4]|32) == 's'
}
