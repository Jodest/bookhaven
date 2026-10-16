package repository

import "testing"

func TestBuildPrefixTSQuery(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"single word", "брат", "брат:*"},
		{"multi word", "власт колец", "власт:* & колец:*"},
		{"extra spaces", "  брат   колец  ", "брат:* & колец:*"},
		{"operators", `a&b | !c (d) e:f* 'g' \h <-> i`, "a:* & b:* & c:* & d:* & e:* & f:* & g:* & h:* & i:*"},
		{"empty", "", ""},
		{"punctuation only", ` &|!():*'\<> `, ""},
		{"digits", "isbn 978", "isbn:* & 978:*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildPrefixTSQuery(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
