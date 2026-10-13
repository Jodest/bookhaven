# home/literary-calendar Specification

## Purpose

Marks authors' birthdays and death anniversaries and round book anniversaries, so the home page has a reason to show older books.

## Requirements

### Requirement: Author dates
The calendar SHALL list published authors with at least one published book whose birth or death day and month equal today's Moscow date; on 28 February of a non-leap year, 29 February dates also match. Each entry carries the event type, the number of years since the event (entries with zero or negative years are skipped) and up to 3 of the author's published books, best rated first. At most 6 entries are shown: anniversaries that are multiples of 25 years first, then authors with more published books, then by name.

#### Scenario: Birthday today
- **WHEN** a published author with two published books was born on today's day and month 200 years ago
- **THEN** the calendar lists the author's birth, 200 years, with both books

### Requirement: Looking ahead
When no author matches today, the calendar SHALL use the nearest of the next 7 days that has matches and say which date it is. When none of the 8 days matches, the calendar has no author entries.

#### Scenario: Nothing today
- **WHEN** no author matches today and one matches in three days
- **THEN** the calendar shows that author under the date three days from now and marks it as not today

### Requirement: Book anniversaries
The calendar SHALL also list up to 6 published books whose publication year is a positive multiple of 25 years before the current year: multiples of 100 first, then multiples of 50, then the rest, each group best rated first. The calendar is `null` when it has neither author entries nor anniversaries.

#### Scenario: Centenary
- **WHEN** it is 2026 and published books from 1926 and 1976 exist
- **THEN** the 1926 book (100 years) is listed before the 1976 book (50 years)

### Requirement: Calendar block
The web block SHALL be titled «Литературный календарь · <day month>» for today's entries or «Скоро в календаре · <day month>» for a later day, and «Литературный календарь» when it only has anniversaries. Each author entry reads «Родился …» or «Умер …» with the years and links to the author page and the books; anniversaries are listed under «Юбилеи года» as ««<title>» — <years>».

#### Scenario: Anniversaries only
- **WHEN** no author matches in the next 8 days and there are anniversaries
- **THEN** the block is titled «Литературный календарь» and lists «Юбилеи года»
