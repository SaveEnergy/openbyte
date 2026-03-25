package api

import "testing"

func TestHasRegistryAPIPrefix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want bool
	}{
		{"/api/v1/registry/servers", true},
		{"/api/v1/registry/", true},
		{"/api/v1/registryx", false},
		{"/api/v1/registry", false},
		{"/", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := hasRegistryAPIPrefix(tc.path); got != tc.want {
			t.Errorf("hasRegistryAPIPrefix(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
