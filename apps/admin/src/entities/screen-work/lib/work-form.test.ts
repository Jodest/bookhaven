import { describe, expect, it } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { deleteWorkErrorText, linkWorkErrorText, saveWorkErrorText } from './error-text';
import {
  EMPTY_WORK_FORM,
  extractImdbId,
  extractKinopoiskId,
  formToInput,
  validateWorkForm,
  type WorkFormState,
} from './work-form';

const form = (patch: Partial<WorkFormState> = {}): WorkFormState => ({
  ...EMPTY_WORK_FORM,
  title: 'Солярис',
  ...patch,
});

describe('validateWorkForm', () => {
  it('accepts a minimal form', () => {
    expect(validateWorkForm(form())).toBeNull();
  });

  it.each([
    [{ title: '  ' }, 'Укажите название.'],
    [{ year: '1850' }, 'Год должен быть от 1880 до 2100.'],
    [{ year: '2101' }, 'Год должен быть от 1880 до 2100.'],
    [{ year: 'abc' }, 'Год должен быть от 1880 до 2100.'],
    [{ imdb_id: 'abc' }, 'IMDb id должен иметь вид tt и цифры.'],
    [{ kinopoisk_id: '12a' }, 'Кинопоиск id должен состоять из цифр.'],
    [{ qid: 'X1' }, 'QID должен иметь вид Q и цифры.'],
  ])('rejects %j', (patch, message) => {
    expect(validateWorkForm(form(patch))).toBe(message);
  });

  it('accepts pasted links and a lowercase qid', () => {
    expect(
      validateWorkForm(
        form({
          imdb_id: 'https://www.imdb.com/title/tt0120737/',
          kinopoisk_id: 'https://www.kinopoisk.ru/film/43911/',
          qid: 'q181069',
          year: '1972',
        })
      )
    ).toBeNull();
  });
});

describe('link extraction', () => {
  it('extracts IMDb ids', () => {
    expect(extractImdbId('https://www.imdb.com/title/tt0120737/?ref_=x')).toBe('tt0120737');
    expect(extractImdbId(' tt0069293 ')).toBe('tt0069293');
  });

  it('extracts Kinopoisk film and series ids', () => {
    expect(extractKinopoiskId('https://www.kinopoisk.ru/film/43911/')).toBe('43911');
    expect(extractKinopoiskId('https://www.kinopoisk.ru/series/77044/')).toBe('77044');
    expect(extractKinopoiskId('43911')).toBe('43911');
  });
});

describe('formToInput', () => {
  it('reduces links, trims and nulls blanks', () => {
    expect(
      formToInput(
        form({
          title: ' Солярис ',
          year: '1972',
          director: '  ',
          kinopoisk_id: 'https://www.kinopoisk.ru/film/43911/',
          qid: 'q1',
        })
      )
    ).toEqual({
      title: 'Солярис',
      original_title: null,
      kind: 'film',
      year: 1972,
      director: null,
      imdb_id: null,
      kinopoisk_id: '43911',
      qid: 'Q1',
    });
  });
});

describe('error texts', () => {
  const err = (status: number, msg: string) => new ApiError(status, '', msg);

  it('maps link errors', () => {
    expect(linkWorkErrorText(err(409, 'screen work is already linked to this book'), 'x')).toBe('Этот фильм уже привязан.');
    expect(linkWorkErrorText(err(409, 'screen work with this imdb id already exists'), 'x')).toBe(
      'Фильм с таким IMDb id уже есть в каталоге — выберите его в поиске.'
    );
    expect(linkWorkErrorText(err(409, 'screen work with this kinopoisk id already exists'), 'x')).toBe(
      'Фильм с таким Кинопоиск id уже есть в каталоге — выберите его в поиске.'
    );
    expect(linkWorkErrorText(err(500, 'boom'), 'Не удалось добавить фильм.')).toBe('Не удалось добавить фильм.');
  });

  it('maps save and delete errors', () => {
    expect(saveWorkErrorText(err(409, 'screen work with this qid already exists'))).toBe(
      'Фильм с таким QID уже есть в каталоге.'
    );
    expect(saveWorkErrorText(err(404, 'screen work not found'))).toBe('Фильм не найден.');
    expect(saveWorkErrorText(err(500, ''))).toBe('Не удалось сохранить фильм.');
    expect(deleteWorkErrorText(err(409, 'screen work has links'))).toBe('Не удалось удалить фильм.');
    expect(deleteWorkErrorText(err(404, ''))).toBe('Фильм не найден.');
  });
});
