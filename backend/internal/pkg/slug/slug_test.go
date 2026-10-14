package slug

import "testing"

func TestMake(t *testing.T) {
	cases := map[string]string{
		"Топ 100 фантастики": "top-100-fantastiki",
		"Ёжик в тумане":      "yozhik-v-tumane",
		"  Hello, World!!  ": "hello-world",
		"Война и мир (1869)": "voyna-i-mir-1869",
		"---":                "",
		"":                   "",
	}
	for in, want := range cases {
		if got := Make(in); got != want {
			t.Errorf("Make(%q) = %q, want %q", in, got, want)
		}
	}
}
