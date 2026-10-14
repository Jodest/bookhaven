const dayMonthFormatter = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric',
  month: 'long',
  timeZone: 'UTC',
});

const moscowDayMonthFormatter = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric',
  month: 'long',
  timeZone: 'Europe/Moscow',
});

/**
 * Formats a civil date `YYYY-MM-DD` as "4 октября". The date is read as UTC
 * midnight and formatted in UTC, so the result never depends on the host
 * timezone (server and client agree).
 */
export function formatDayMonth(date: string): string {
  return dayMonthFormatter.format(new Date(`${date.slice(0, 10)}T00:00:00Z`));
}

/** Formats an ISO timestamp as "4 октября" in Moscow time (the site's day). */
export function formatTimestampDayMonth(iso: string): string {
  return moscowDayMonthFormatter.format(new Date(iso));
}

/**
 * Picks the Russian plural form for n: forms are [one, few, many], e.g.
 * ['год', 'года', 'лет'].
 */
export function pluralRu(n: number, forms: [string, string, string]): string {
  const abs = Math.abs(n);
  const mod10 = abs % 10;
  const mod100 = abs % 100;
  if (mod10 === 1 && mod100 !== 11) return forms[0];
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return forms[1];
  return forms[2];
}

/** "1 год", "2 года", "212 лет". */
export function formatYears(n: number): string {
  return `${n} ${pluralRu(n, ['год', 'года', 'лет'])}`;
}

/** "4,6 · 12 оценок"; null when the book has no ratings. */
export function formatRatingSummary(rating: number, ratingsCount: number): string | null {
  if (ratingsCount <= 0) return null;
  const value = rating.toLocaleString('ru-RU', { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  return `${value} · ${ratingsCount} ${pluralRu(ratingsCount, ['оценка', 'оценки', 'оценок'])}`;
}

const moscowLongDateFormatter = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
  timeZone: 'Europe/Moscow',
});

/** Formats an ISO timestamp as "14 октября 2026" in Moscow time (the site's day). */
export function formatTimestampLongDate(iso: string): string {
  return moscowLongDateFormatter.format(new Date(iso)).replace(/\s*г\.$/, '');
}
