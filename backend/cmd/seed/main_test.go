package main

import "testing"

func TestShouldRefuseToRun(t *testing.T) {
	cases := []struct {
		name   string
		env    string
		force  bool
		refuse bool
	}{
		{"production without force refuses", "production", false, true},
		{"production with force allowed", "production", true, false},
		{"development without force allowed", "development", false, false},
		{"development with force allowed", "development", true, false},
		{"empty env allowed", "", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldRefuseToRun(tc.env, tc.force)
			if got != tc.refuse {
				t.Errorf("shouldRefuseToRun(%q, %v) = %v, want %v", tc.env, tc.force, got, tc.refuse)
			}
		})
	}
}
