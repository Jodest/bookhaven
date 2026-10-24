import { pluralRu } from './plural-ru';

// Intl "ru" gives the genitive case, so nominative lowercase names are fixed here.
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

/** «Прочитано N раз» with the Russian plural of «раз». */
export function formatReadCount(n: number): string {
  return `Прочитано ${n} ${pluralRu(n, ['раз', 'раза', 'раз'])}`;
}

/** «Прочитано N раз · последний раз — <месяц год>» (local time). */
export function formatReadCounter(n: number, lastReadAt: string | null | undefined): string {
  const base = formatReadCount(n);
  if (!lastReadAt) return base;
  const date = new Date(lastReadAt);
  if (Number.isNaN(date.getTime())) return base;
  return `${base} · последний раз — ${MONTHS_NOMINATIVE[date.getMonth()]} ${date.getFullYear()}`;
}
