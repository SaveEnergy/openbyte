package api

import "testing"

func TestForwardedProtoIsHTTPS(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"https", true},
		{"HTTPS", true},
		{"HttpS", true},
		{"http", false},
		{"HTTP", false},
		{"https ", false},
		{" https", false},
		{"httpss", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := forwardedProtoIsHTTPS(tc.in); got != tc.want {
			t.Errorf("forwardedProtoIsHTTPS(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
