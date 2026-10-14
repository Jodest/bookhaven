/** «Источник: …» line text; null when the collection names no source. */
export function formatSource(source: string | null | undefined): string | null {
  const text = source?.trim();
  return text ? `Источник: ${text}` : null;
}

/** «{n} книг» with the Russian plural form. */
export function formatBooksCount(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return `${count} книга`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return `${count} книги`;
  return `${count} книг`;
}
