package amount

import "testing"

func TestParseSmallestUnits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value string
		want  int64
		ok    bool
	}{
		{value: "12.340000", want: 12_340_000, ok: true},
		{value: "0.000001", want: 1, ok: true},
		{value: "42", want: 42_000_000, ok: true},
		{value: "9223372036854.775807", want: 9_223_372_036_854_775_807, ok: true},
		{value: "9223372036854.775808", ok: false},
		{value: "0", ok: false},
		{value: "1.0000001", ok: false},
		{value: "9223372036854775807", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := ParseSmallestUnits(tt.value)
			if (err == nil) != tt.ok {
				t.Fatalf("ParseSmallestUnits(%q) error = %v", tt.value, err)
			}
			if tt.ok && got != tt.want {
				t.Fatalf("ParseSmallestUnits(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}
