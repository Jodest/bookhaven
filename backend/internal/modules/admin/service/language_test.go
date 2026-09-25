package service

import "testing"

func TestIsRussianText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"pure Cyrillic", "Война и мир", true},
		{"pure Latin", "War and Peace", false},
		{"mixed, majority Cyrillic (above threshold)", "Изучаем Go", true},
		{"mixed, majority Latin (below threshold)", "Learning Го", false},
		{"empty string", "", true},
		{"digits and punctuation only", "12345 - !?", true},
		// Cyrillic 'а' (U+0430) + Latin 'b': exactly 1/2 Cyrillic. The
		// threshold check is >=, so an exact-boundary ratio passes.
		{"exactly at threshold (0.5) passes", "аb", true},
		{"clearly below threshold, mostly Latin", "аbcd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRussianText(tt.in); got != tt.want {
				t.Errorf("IsRussianText(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
