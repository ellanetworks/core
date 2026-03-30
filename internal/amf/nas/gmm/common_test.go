package gmm

import "testing"

func TestNextNgKsi(t *testing.T) {
	tests := []struct {
		name    string
		current int32
		want    int32
	}{
		{"0 -> 1", 0, 1},
		{"3 -> 4", 3, 4},
		{"5 -> 6", 5, 6},
		{"6 wraps to 0", 6, 0},
		{"7 (no key) wraps to 0", 7, 0},
		{"8 (out of range) wraps to 0", 8, 0},
		{"negative wraps to 0", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextNgKsi(tt.current)
			if got != tt.want {
				t.Errorf("nextNgKsi(%d) = %d, want %d", tt.current, got, tt.want)
			}
		})
	}
}
