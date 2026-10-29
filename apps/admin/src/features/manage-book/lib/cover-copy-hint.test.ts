import { describe, expect, it } from 'vitest';
import { coverCopyHint } from './cover-copy-hint';

const copy = (status: 'pending' | 'stored' | 'failed' | 'absent' | 'linked', error: string | null = null) => ({
  status,
  error,
  stored_at: null,
});

describe('coverCopyHint', () => {
  it('has no hint without a copy object', () => {
    expect(coverCopyHint(null)).toBeNull();
    expect(coverCopyHint(undefined)).toBeNull();
  });

  it('stored', () => {
    expect(coverCopyHint(copy('stored'))).toBe('Копия обложки сохранена на нашем сервере.');
  });

  it('pending', () => {
    expect(coverCopyHint(copy('pending'))).toBe(
      'Обложка копируется на наш сервер. Пока показывается исходная ссылка.'
    );
  });

  it('absent', () => {
    expect(coverCopyHint(copy('absent'))).toBe(
      'По этой ссылке нет обложки. На сайте показывается «Нет обложки».'
    );
  });

  it('linked', () => {
    expect(coverCopyHint(copy('linked'))).toBe(
      'Обложка показывается по ссылке с сайта-источника. Копия на нашем сервере не сохраняется.'
    );
  });

  it.each([
    ['host_not_allowed', 'сайт не входит в список разрешённых'],
    ['blocked_address', 'сайт не входит в список разрешённых'],
    ['too_large', 'файл больше 5 МБ'],
    ['not_image', 'это не изображение JPEG, PNG или WebP'],
    ['http_error', 'источник недоступен'],
    ['timeout', 'источник недоступен'],
    ['network', 'источник недоступен'],
    ['too_many_redirects', 'источник недоступен'],
    [null, 'источник недоступен'],
  ])('failed with %s', (error, reason) => {
    expect(coverCopyHint(copy('failed', error))).toBe(
      `Не удалось скопировать обложку: ${reason}. На сайте показывается исходная ссылка.`
    );
  });
});
