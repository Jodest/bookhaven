package service

import (
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/home/repository"
	"github.com/google/uuid"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func pool(n int) []uuid.UUID {
	ids := make([]uuid.UUID, n)
	for i := range ids {
		ids[i] = uuid.New()
	}
	return ids
}

func TestPickDaily_Deterministic(t *testing.T) {
	p := pool(20)
	d := day(2026, 10, 4)
	a, ok1 := pickDaily(p, d, "botd")
	b, ok2 := pickDaily(p, d, "botd")
	if !ok1 || !ok2 || a != b {
		t.Fatalf("expected the same pick, got %v %v", a, b)
	}
}

func TestPickDaily_SaltChangesSequence(t *testing.T) {
	p := pool(50)
	start := day(2026, 1, 1)
	diff := 0
	for i := 0; i < 30; i++ {
		d := start.AddDate(0, 0, i)
		a, _ := pickDaily(p, d, "botd")
		b, _ := pickDaily(p, d, "wtr:user")
		if a != b {
			diff++
		}
	}
	if diff == 0 {
		t.Fatal("expected different salts to pick differently on at least some days")
	}
}

func TestPickDaily_NoRepeatOnConsecutiveDays(t *testing.T) {
	for _, size := range []int{2, 3, 50} {
		p := pool(size)
		start := day(2026, 9, 1)
		prev, _ := pickDaily(p, start, "botd")
		for i := 1; i <= 60; i++ {
			cur, ok := pickDaily(p, start.AddDate(0, 0, i), "botd")
			if !ok {
				t.Fatalf("size %d: expected a pick", size)
			}
			if cur == prev {
				t.Fatalf("size %d: repeated pick on day %d", size, i)
			}
			prev = cur
		}
	}
}

func TestPickDaily_InputOrderIrrelevant(t *testing.T) {
	p := pool(10)
	rev := make([]uuid.UUID, len(p))
	for i := range p {
		rev[len(p)-1-i] = p[i]
	}
	for i := 0; i < 10; i++ {
		d := day(2026, 10, 1).AddDate(0, 0, i)
		a, _ := pickDaily(p, d, "botd")
		b, _ := pickDaily(rev, d, "botd")
		if a != b {
			t.Fatalf("day %d: order changed the pick", i)
		}
	}
}

func TestPickDaily_SingleAndEmpty(t *testing.T) {
	only := uuid.New()
	if got, ok := pickDaily([]uuid.UUID{only}, day(2026, 10, 4), "botd"); !ok || got != only {
		t.Fatalf("expected the single element, got %v ok=%v", got, ok)
	}
	if got, ok := pickDaily(nil, day(2026, 10, 4), "botd"); ok || got != uuid.Nil {
		t.Fatalf("expected Nil/false, got %v ok=%v", got, ok)
	}
}

func TestCalendarDays(t *testing.T) {
	cases := []struct {
		name string
		d    time.Time
		want []repository.MonthDay
	}{
		{"normal day", day(2026, 10, 4), []repository.MonthDay{{Month: 10, Day: 4}}},
		{"feb 28 non-leap", day(2027, 2, 28), []repository.MonthDay{{Month: 2, Day: 28}, {Month: 2, Day: 29}}},
		{"feb 28 leap", day(2028, 2, 28), []repository.MonthDay{{Month: 2, Day: 28}}},
		{"feb 29 leap", day(2028, 2, 29), []repository.MonthDay{{Month: 2, Day: 29}}},
		{"mar 1 non-leap", day(2027, 3, 1), []repository.MonthDay{{Month: 3, Day: 1}}},
	}
	for _, c := range cases {
		got := calendarDays(c.d)
		if len(got) != len(c.want) {
			t.Fatalf("%s: got %v want %v", c.name, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("%s: got %v want %v", c.name, got, c.want)
			}
		}
	}
}

func TestIsLeap(t *testing.T) {
	for y, want := range map[int]bool{2024: true, 2026: false, 2028: true, 1900: false, 2000: true} {
		if isLeap(y) != want {
			t.Fatalf("isLeap(%d) != %v", y, want)
		}
	}
}

func TestCivilDate_MoscowNotUTC(t *testing.T) {
	got := civilDate(time.Date(2026, 10, 3, 22, 30, 0, 0, time.UTC))
	if dateString(got) != "2026-10-04" {
		t.Fatalf("expected 2026-10-04 in MSK, got %s", dateString(got))
	}
	got = civilDate(time.Date(2026, 10, 3, 20, 59, 0, 0, time.UTC))
	if dateString(got) != "2026-10-03" {
		t.Fatalf("expected 2026-10-03 in MSK, got %s", dateString(got))
	}
}

func TestEventYears(t *testing.T) {
	if got := eventYears(day(1814, 10, 15), 2026); got != 212 {
		t.Fatalf("expected 212, got %d", got)
	}
	if got := eventYears(day(2030, 1, 1), 2026); got > 0 {
		t.Fatalf("expected a non-positive value for a future date, got %d", got)
	}
}

func TestOrderCalendarEvents(t *testing.T) {
	ev := func(name string, years, books int) calendarEvent {
		return calendarEvent{Author: repository.CalendarEventRow{Name: name, VisibleBooks: books}, Years: years}
	}
	events := []calendarEvent{
		ev("Б", 212, 5),
		ev("А", 212, 5),
		ev("В", 213, 9),
		ev("Г", 200, 1),
	}
	orderCalendarEvents(events)
	got := []string{}
	for _, e := range events {
		got = append(got, e.Author.Name)
	}
	if strings.Join(got, ",") != "Г,В,А,Б" {
		t.Fatalf("unexpected order %v", got)
	}
}

func TestIsAnniversaryAndRank(t *testing.T) {
	for _, age := range []int{25, 50, 75, 100, 150} {
		if !isAnniversary(age, AnniversaryStep) {
			t.Fatalf("expected %d to be an anniversary", age)
		}
	}
	for _, age := range []int{0, 10, -25} {
		if isAnniversary(age, AnniversaryStep) {
			t.Fatalf("expected %d not to be an anniversary", age)
		}
	}
	if !(anniversaryRank(100) < anniversaryRank(50) && anniversaryRank(50) < anniversaryRank(25)) {
		t.Fatal("expected 100 > 50 > 25 in rank")
	}
	if anniversaryRank(150) != anniversaryRank(50) {
		t.Fatal("150 is a multiple of 50")
	}
}

func TestExcerpt(t *testing.T) {
	if got := excerpt("Коротко.", 280); got != "Коротко." {
		t.Fatalf("short text changed: %q", got)
	}

	if got := excerpt("one two three four", 10); got != "one two…" {
		t.Fatalf("word boundary: %q", got)
	}

	// 10 Cyrillic letters = 20 bytes; a byte-based cut would differ.
	cyr := "абвгд еёжзи клмно"
	if got := excerpt(cyr, 12); got != "абвгд еёжзи…" {
		t.Fatalf("cyrillic runes: %q", got)
	}

	long := strings.Repeat("я", 50)
	got := excerpt(long, 10)
	if got != strings.Repeat("я", 10)+"…" {
		t.Fatalf("hard cut: %q", got)
	}

	if got := excerpt("Первое предложение, второе продолжение", 21); got != "Первое предложение…" {
		t.Fatalf("trailing punctuation: %q", got)
	}
}
