package results

import "testing"

func TestValidResultID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id   string
		want bool
	}{
		{"a1b2c3d4", true},
		{"ABCD1234", true},
		{"00000000", true},
		{"a1b2c3d", false},
		{"a1b2c3d45", false},
		{"a1b2c3d!", false},
		{"a1b2c3ä4", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := validResultID(tc.id); got != tc.want {
			t.Errorf("validResultID(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}
