package logging_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
)

type testStringer struct{}

func (testStringer) String() string {
	return "stringer-value"
}

func TestFormatValueTypes(t *testing.T) {
	now := time.Date(2026, 1, 16, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{name: "string", input: "hello", want: "hello"},
		{name: "bool", input: true, want: "true"},
		{name: "int", input: 42, want: "42"},
		{name: "uint", input: uint(9), want: "9"},
		{name: "float32", input: float32(1.5), want: "1.50"},
		{name: "float64", input: 2.25, want: "2.25"},
		{name: "duration", input: 1500 * time.Millisecond, want: "1.5s"},
		{name: "time", input: now, want: now.Format(time.RFC3339Nano)},
		{name: "stringer", input: testStringer{}, want: "stringer-value"},
		{name: "fallback", input: []int{1, 2}, want: fmt.Sprintf("%v", []int{1, 2})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := logging.FormatValue(tt.input); got != tt.want {
				t.Fatalf("FormatValue got=%q want=%q", got, tt.want)
			}
		})
	}
}
