import { describe, expect, it } from 'vitest';
import {
  curationErrorMessage,
  curationStatusLabel,
  isCurationActive,
  matchStatusLabel,
} from './labels';

describe('ai-curation labels', () => {
  it.each([
    ['matched', 'Найден'],
    ['ambiguous', 'Неоднозначно'],
    ['not_found', 'Не найден'],
    ['error', 'Ошибка Wikidata'],
    ['pending', 'Проверяется…'],
    ['something_new', 'something_new'],
  ])('match status %s -> %s', (status, label) => {
    expect(matchStatusLabel(status)).toBe(label);
  });

  it('never labels a match as confirmed', () => {
    expect(matchStatusLabel('matched')).not.toMatch(/Подтвержд/);
  });

  it.each(['model_refusal', 'model_truncated', 'invalid_output', 'model_error', 'enqueue_failed', 'timeout'])(
    'error code %s has a readable message',
    (code) => {
      expect(curationErrorMessage(code)).not.toBe(code);
      expect(curationErrorMessage(code)).toMatch(/[А-Яа-я]/);
    }
  );

  it('shows an unknown error code raw', () => {
    expect(curationErrorMessage('brand_new_code')).toBe('brand_new_code');
  });

  it('status labels and activity', () => {
    expect(curationStatusLabel('running')).toBe('Выполняется');
    expect(curationStatusLabel('weird')).toBe('weird');
    expect(isCurationActive('queued')).toBe(true);
    expect(isCurationActive('running')).toBe(true);
    expect(isCurationActive('done')).toBe(false);
    expect(isCurationActive('failed')).toBe(false);
  });
});
