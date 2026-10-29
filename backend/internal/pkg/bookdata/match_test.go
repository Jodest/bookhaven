package bookdata

import "testing"

func TestTitleMatches(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		wanted    []string
		want      bool
	}{
		{"unrelated", "I, Robot", []string{"История Петра I"}, false},
		{"other pushkin title", "Евгений Онегин", []string{"Капитанская дочка"}, false},
		{"case", "белый клык", []string{"Белый Клык"}, true},
		{"subtitle colon", "Зов предков: повесть", []string{"Зов предков"}, true},
		{"yo", "Ёлка", []string{"Елка"}, true},
		{"alt title", "White Fang", []string{"Белый Клык", "White Fang"}, true},
		{"empty wanted", "White Fang", []string{"", " "}, false},
		{"no wanted", "White Fang", nil, false},
		{"empty candidate", "", []string{"X"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TitleMatches(tt.candidate, tt.wanted); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestAuthorMatches(t *testing.T) {
	tests := []struct {
		name      string
		candidate []string
		wanted    []string
		want      bool
	}{
		{"asimov vs pushkin", []string{"Isaac Asimov"}, []string{"Александр Пушкин", "Alexander Pushkin"}, false},
		{"english label", []string{"Jack London"}, []string{"Джек Лондон", "Jack London"}, true},
		{"russian", []string{"А. С. Пушкин"}, []string{"Александр Пушкин"}, true},
		{"empty candidate", nil, []string{"Лондон"}, false},
		{"empty wanted", []string{"Jack London"}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AuthorMatches(tt.candidate, tt.wanted); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
