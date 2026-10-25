import type { BookRead } from '@bookhaven/api-client';

const MONTHS_NOMINATIVE = [
  'январь',
  'февраль',
  'март',
  'апрель',
  'май',
  'июнь',
  'июль',
  'август',
  'сентябрь',
  'октябрь',
  'ноябрь',
  'декабрь',
] as const;

const MONTHS_GENITIVE = [
  'января',
  'февраля',
  'марта',
  'апреля',
  'мая',
  'июня',
  'июля',
  'августа',
  'сентября',
  'октября',
  'ноября',
  'декабря',
] as const;

function monthYear(iso: string): string {
  const d = new Date(iso);
  return `${MONTHS_NOMINATIVE[d.getMonth()]} ${d.getFullYear()}`;
}

/** «<N>-е прочтение: <period>» in the reader's local time. */
export function formatReadingLine(index: number, read: BookRead): string {
  const prefix = `${index}-е прочтение: `;
  if (read.state === 'open') {
    if (!read.startedAt) return `${prefix}читаю`;
    const d = new Date(read.startedAt);
    return `${prefix}с ${MONTHS_GENITIVE[d.getMonth()]} ${d.getFullYear()} — читаю`;
  }
  const end = read.state === 'finished' ? read.finishedAt : read.abandonedAt;
  let period = '';
  if (end) {
    period = read.startedAt ? `${monthYear(read.startedAt)} — ${monthYear(end)}` : monthYear(end);
  } else if (read.startedAt) {
    period = monthYear(read.startedAt);
  }
  return `${prefix}${period}${read.state === 'abandoned' ? ', брошено' : ''}`;
}

function pad(n: number): string {
  return String(n).padStart(2, '0');
}

/** The local calendar day of an ISO time as YYYY-MM-DD (for <input type="date">). */
export function toLocalDateInput(iso: string): string {
  const d = new Date(iso);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

/** Today's local calendar day as YYYY-MM-DD. */
export function todayLocalDateInput(): string {
  return toLocalDateInput(new Date().toISOString());
}
