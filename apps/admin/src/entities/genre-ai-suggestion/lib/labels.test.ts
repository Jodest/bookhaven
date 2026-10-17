import { describe, expect, it } from 'vitest';
import type { AdminGenreAIRun } from '@bookhaven/api-client';
import {
  confidencePercent,
  isSuggestionKind,
  isSuggestionStatus,
  lastRunText,
  reasonCodeLabel,
  suggestionActionLabel,
  suggestionKindLabel,
} from './labels';

const run = (over: Partial<AdminGenreAIRun>): AdminGenreAIRun => ({
  id: 'r1',
  trigger: 'manual',
  status: 'done',
  batches_total: 2,
  batches_done: 2,
  suggestions_created: 3,
  error_code: null,
  created_at: '2026-10-15T10:00:00Z',
  finished_at: '2026-10-15T10:05:00Z',
  ...over,
});

const fmt = (iso: string) => `<${iso.slice(0, 16)}>`;

describe('genre AI suggestion labels', () => {
  it('labels kinds and reason codes in Russian', () => {
    expect(suggestionKindLabel('duplicate')).toBe('Дубликат');
    expect(suggestionKindLabel('narrow')).toBe('Узкий → общий');
    expect(suggestionKindLabel('junk')).toBe('Не жанр');
    expect(reasonCodeLabel('spelling')).toBe('написание');
    expect(reasonCodeLabel('language')).toBe('другой язык');
    expect(reasonCodeLabel('synonym')).toBe('синоним');
    expect(reasonCodeLabel('too_narrow')).toBe('слишком узкий');
    expect(reasonCodeLabel('not_a_genre')).toBe('не жанр');
    expect(reasonCodeLabel('date')).toBe('дата');
    expect(reasonCodeLabel('marketing')).toBe('маркетинг');
    expect(reasonCodeLabel('format')).toBe('формат издания');
  });

  it('shows an unknown value raw', () => {
    expect(suggestionKindLabel('x')).toBe('x');
    expect(reasonCodeLabel('weird')).toBe('weird');
  });

  it('builds the row action', () => {
    expect(suggestionActionLabel('duplicate', 'Фэнтези')).toBe('Слить в «Фэнтези»');
    expect(suggestionActionLabel('narrow', 'Киберпанк')).toBe('Слить в «Киберпанк»');
    expect(suggestionActionLabel('junk', null)).toBe('Отклонить жанр');
  });

  it('formats confidence as whole percent', () => {
    expect(confidencePercent(0.934)).toBe('93%');
    expect(confidencePercent(1)).toBe('100%');
    expect(confidencePercent(0.5)).toBe('50%');
  });

  it('validates URL filter values', () => {
    expect(isSuggestionStatus('stale')).toBe(true);
    expect(isSuggestionStatus('nope')).toBe(false);
    expect(isSuggestionStatus(null)).toBe(false);
    expect(isSuggestionKind('junk')).toBe(true);
    expect(isSuggestionKind('other')).toBe(false);
  });

  it('describes the last run', () => {
    expect(lastRunText(null, fmt)).toBe('Анализ ещё не запускался.');
    expect(lastRunText(run({}), fmt)).toBe('Последний анализ: <2026-10-15T10:05>');
    expect(lastRunText(run({ status: 'running', finished_at: null }), fmt)).toBe(
      'Последний анализ: <2026-10-15T10:00>, идёт…'
    );
    expect(lastRunText(run({ status: 'queued', finished_at: null }), fmt)).toContain('идёт…');
    expect(lastRunText(run({ status: 'failed', error_code: 'transient' }), fmt)).toBe(
      'Последний анализ: <2026-10-15T10:05>, завершился с ошибкой'
    );
  });
});
