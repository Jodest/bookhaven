// Formats a rate-limit wait (seconds) for "Попробуйте снова через N ...".
// Abbreviated units avoid Russian plural forms ("1 минута / 2 минуты / 5 минут").
export function formatRetryAfter(seconds: number): string {
  const s = Math.max(1, Math.ceil(seconds));
  if (s < 60) return `${s} сек`;
  if (s < 3600) return `${Math.ceil(s / 60)} мин`;
  return `${Math.ceil(s / 3600)} ч`;
}
