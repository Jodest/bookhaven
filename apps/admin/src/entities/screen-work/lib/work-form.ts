import type { AdminScreenWork, ScreenWorkInput, ScreenWorkKind } from '@bookhaven/api-client';

// Form state, validation and payload of a screen work typed by an admin
// (openspec screen-works/admin-editing). The server validates again.

export interface WorkFormState {
  title: string;
  original_title: string;
  kind: ScreenWorkKind;
  year: string;
  director: string;
  imdb_id: string;
  kinopoisk_id: string;
  qid: string;
}

export const EMPTY_WORK_FORM: WorkFormState = {
  title: '',
  original_title: '',
  kind: 'film',
  year: '',
  director: '',
  imdb_id: '',
  kinopoisk_id: '',
  qid: '',
};

export function workToForm(work: AdminScreenWork): WorkFormState {
  return {
    title: work.title,
    original_title: work.original_title ?? '',
    kind: work.kind,
    year: work.year === null ? '' : String(work.year),
    director: work.director ?? '',
    imdb_id: work.imdb_id ?? '',
    kinopoisk_id: work.kinopoisk_id ?? '',
    qid: work.qid ?? '',
  };
}

/** Accepts a bare id or an imdb.com/title/tt… link. */
export function extractImdbId(raw: string): string {
  const v = raw.trim();
  const m = v.match(/imdb\.com\/title\/(tt\d+)/i);
  return m ? m[1] : v;
}

/** Accepts a bare id or a kinopoisk.ru/film|series/<n> link. */
export function extractKinopoiskId(raw: string): string {
  const v = raw.trim();
  const m = v.match(/kinopoisk\.ru\/(?:film|series)\/(\d+)/i);
  return m ? m[1] : v;
}

const MAX_TEXT = 500;
const MAX_EXTERNAL = 20;

/** First violation as a Russian message, or null when the form is valid. */
export function validateWorkForm(form: WorkFormState): string | null {
  if (form.title.trim() === '') return 'Укажите название.';
  if (form.title.trim().length > MAX_TEXT) return 'Название слишком длинное.';
  if (form.original_title.trim().length > MAX_TEXT) return 'Оригинальное название слишком длинное.';
  if (form.director.trim().length > MAX_TEXT) return 'Поле «Режиссёр» слишком длинное.';
  const year = form.year.trim();
  if (year !== '') {
    const n = Number(year);
    if (!/^\d+$/.test(year) || n < 1880 || n > 2100) return 'Год должен быть от 1880 до 2100.';
  }
  const imdb = extractImdbId(form.imdb_id);
  if (imdb !== '' && (!/^tt\d+$/.test(imdb) || imdb.length > MAX_EXTERNAL)) {
    return 'IMDb id должен иметь вид tt и цифры.';
  }
  const kp = extractKinopoiskId(form.kinopoisk_id);
  if (kp !== '' && (!/^\d+$/.test(kp) || kp.length > MAX_EXTERNAL)) {
    return 'Кинопоиск id должен состоять из цифр.';
  }
  const qid = form.qid.trim().toUpperCase();
  if (qid !== '' && !/^Q\d+$/.test(qid)) return 'QID должен иметь вид Q и цифры.';
  return null;
}

const orNull = (v: string): string | null => {
  const t = v.trim();
  return t === '' ? null : t;
};

/** API payload; links are reduced to ids. Call after validateWorkForm. */
export function formToInput(form: WorkFormState): ScreenWorkInput {
  const year = form.year.trim();
  return {
    title: form.title.trim(),
    original_title: orNull(form.original_title),
    kind: form.kind,
    year: year === '' ? null : Number(year),
    director: orNull(form.director),
    imdb_id: orNull(extractImdbId(form.imdb_id)),
    kinopoisk_id: orNull(extractKinopoiskId(form.kinopoisk_id)),
    qid: orNull(form.qid.toUpperCase()),
  };
}
