package domain

import (
	"strings"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name, in, title, author string
		ok                      bool
	}{
		{"numbered with author", "12) «Мастер и Маргарита» — Михаил Булгаков", "Мастер и Маргарита", "Михаил Булгаков", true},
		{"dot numbering", "1. Мать - Горький", "Мать", "Горький", true},
		{"number sign", "№ 3. Мать", "Мать", "", true},
		{"number is title", "1984 — Джордж Оруэлл", "1984", "Джордж Оруэлл", true},
		{"bare number title", "1984", "1984", "", true},
		{"tab with numbering", "3\tПикник на обочине\tСтругацкие", "Пикник на обочине", "Стругацкие", true},
		{"two tab cells", "Мать\tГорький", "Мать", "Горький", true},
		{"slash", "Мать / Горький", "Мать", "Горький", true},
		{"en dash", "Мать – Горький", "Мать", "Горький", true},
		{"last dash wins", "Москва - Петушки - Ерофеев", "Москва - Петушки", "Ерофеев", true},
		{"quotes and punctuation", `"Мать".  — „Горький“;`, "Мать", "Горький", true},
		{"title only", "Война и мир", "Война и мир", "", true},
		{"numbering only", "7.", "", "", false},
		{"empty after quotes", "«»", "", "", false},
		{"unspaced dash is part of title", "Кто-то пришёл", "Кто-то пришёл", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			title, author, ok := ParseLine(c.in)
			if title != c.title || author != c.author || ok != c.ok {
				t.Fatalf("ParseLine(%q) = (%q, %q, %v), want (%q, %q, %v)", c.in, title, author, ok, c.title, c.author, c.ok)
			}
		})
	}
}

func TestParseWholeRetryText(t *testing.T) {
	p := Parse("Москва - Петушки")
	if p.Title != "Москва" || p.Author != "Петушки" || p.Whole != "Москва - Петушки" {
		t.Fatalf("unexpected parse: %+v", p)
	}
}

func TestParseTruncatesLongLine(t *testing.T) {
	p := Parse(strings.Repeat("я", 800))
	if n := len([]rune(p.Title)); n != MaxPasteLineRunes {
		t.Fatalf("title runes = %d, want %d", n, MaxPasteLineRunes)
	}
}

func TestMatchAuthor(t *testing.T) {
	cases := []struct {
		pasted  string
		authors []string
		want    bool
	}{
		{"Булгаков", []string{"Михаил Булгаков"}, true},
		{"М. Булгаков", []string{"Михаил Булгаков"}, true},
		{"Михаил Булгаков", []string{"Михаил Булгаков"}, true},
		{"Н. Булгаков", []string{"Михаил Булгаков"}, false},
		{"Толстой", []string{"Максим Горький"}, false},
		{"Горький и Чапек", []string{"Карел Чапек"}, true},
		{"Горький, Чапек", []string{"Карел Чапек"}, true},
		{"Ильф & Петров", []string{"Евгений Петров"}, true},
		{"Стругацкие", []string{"Аркадий Стругацкий"}, false},
		{"горький", []string{"Максим ГОРЬКИЙ"}, true},
		{"", []string{"Максим Горький"}, false},
		{"Горький", nil, false},
	}
	for _, c := range cases {
		if got := MatchAuthor(c.pasted, c.authors); got != c.want {
			t.Errorf("MatchAuthor(%q, %v) = %v, want %v", c.pasted, c.authors, got, c.want)
		}
	}
}

func book(title, status string, authors ...string) catalogDomain.Book {
	b := catalogDomain.Book{ID: uuid.New(), Title: title, Status: status}
	for _, a := range authors {
		b.Authors = append(b.Authors, catalogDomain.Author{ID: uuid.New(), Name: a})
	}
	return b
}

func TestClassify(t *testing.T) {
	pub, pend, rej := catalogDomain.BookStatusPublished, catalogDomain.BookStatusPending, catalogDomain.BookStatusRejected
	gorky := book("Мать", pub, "Максим Горький")
	capek := book("Мать", pub, "Карел Чапек")
	rejected := book("Мать", rej, "Кто-то")
	pending := book("Мать", pend, "Максим Горький")

	t.Run("not found", func(t *testing.T) {
		if got := Classify(nil, "", nil); got.Status != PasteNotFound {
			t.Fatalf("status = %s", got.Status)
		}
	})
	t.Run("single found", func(t *testing.T) {
		got := Classify([]catalogDomain.Book{gorky}, "", nil)
		if got.Status != PasteFound || got.Book == nil || got.Book.ID != gorky.ID {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("pending found", func(t *testing.T) {
		got := Classify([]catalogDomain.Book{pending}, "", nil)
		if got.Status != PasteFound || got.Book.Status != pend {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("same title different authors", func(t *testing.T) {
		got := Classify([]catalogDomain.Book{capek, gorky}, "", nil)
		if got.Status != PasteAmbiguous || len(got.Candidates) != 2 {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("author narrows", func(t *testing.T) {
		got := Classify([]catalogDomain.Book{capek, gorky}, "М. Горький", nil)
		if got.Status != PasteFound || got.Book.ID != gorky.ID {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("author mismatch falls back to ambiguous", func(t *testing.T) {
		got := Classify([]catalogDomain.Book{gorky}, "Толстой", nil)
		if got.Status != PasteAmbiguous || len(got.Candidates) != 1 || got.Candidates[0].ID != gorky.ID {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("only rejected", func(t *testing.T) {
		got := Classify([]catalogDomain.Book{rejected}, "", nil)
		if got.Status != PasteRejected || len(got.Candidates) != 1 || got.Candidates[0].Status != rej {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("rejected does not count", func(t *testing.T) {
		got := Classify([]catalogDomain.Book{rejected, gorky}, "", nil)
		if got.Status != PasteFound || got.Book.ID != gorky.ID {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("in collection", func(t *testing.T) {
		got := Classify([]catalogDomain.Book{gorky}, "", map[uuid.UUID]bool{gorky.ID: true})
		if got.Status != PasteInCollection || got.Book == nil {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("one of two in collection is ambiguous", func(t *testing.T) {
		got := Classify([]catalogDomain.Book{gorky, capek}, "", map[uuid.UUID]bool{gorky.ID: true})
		if got.Status != PasteAmbiguous {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("candidates ordered and capped", func(t *testing.T) {
		var many []catalogDomain.Book
		many = append(many, rejected)
		for i := 0; i < 12; i++ {
			many = append(many, book("Мать", pub, "A"+string(rune('a'+i))))
		}
		many = append(many, pending)
		got := Classify(many, "", nil)
		if got.Status != PasteAmbiguous || len(got.Candidates) != MaxCandidates {
			t.Fatalf("got %s with %d candidates", got.Status, len(got.Candidates))
		}
		for _, c := range got.Candidates {
			if c.Status != pub {
				t.Fatalf("published must come first, got %s", c.Status)
			}
		}
	})
}

func TestMarkDuplicates(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	got := MarkDuplicates([]PasteEntry{
		{Line: 1, Key: "мать горький", Status: PasteFound, BookID: a},
		{Line: 2, Key: "война и мир", Status: PasteFound, BookID: b},
		{Line: 4, Key: "другая запись", Status: PasteFound, BookID: a},
		{Line: 5, Key: "война и мир", Status: PasteNotFound},
		{Line: 6, Key: "", Status: PasteInvalid},
		{Line: 7, Key: "х", Status: PasteNotFound},
		{Line: 8, Key: "х", Status: PasteNotFound},
	})
	want := []int{0, 0, 1, 2, 0, 0, 7}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("duplicate_of = %v, want %v", got, want)
		}
	}
}
