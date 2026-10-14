package textnorm

import "testing"

func TestKey(t *testing.T) {
	cases := map[string]string{
		"":                     "",
		"  Ёлка:  повесть! ":   "елка повесть",
		"Москва - Петушки":     "москва петушки",
		"«Мастер и Маргарита»": "мастер и маргарита",
		"1984":                 "1984",
		"Hello,World":          "hello world",
		"---":                  "",
	}
	for in, want := range cases {
		if got := Key(in); got != want {
			t.Errorf("Key(%q) = %q, want %q", in, got, want)
		}
	}
}
