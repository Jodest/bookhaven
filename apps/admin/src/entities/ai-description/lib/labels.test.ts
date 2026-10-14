import { describe, expect, it } from 'vitest';
import {
  aiDescriptionErrorLabel,
  aiDescriptionStatusLabel,
  isAIDescriptionStatus,
  STATUS_FILTER_LABELS,
} from './labels';

describe('ai-description labels', () => {
  it.each([
    ['draft', 'Черновик'],
    ['approved', 'Одобрено'],
    ['discarded', 'Отклонено'],
    ['unknown', 'ИИ не знает книгу'],
    ['failed', 'Ошибка'],
    ['brand_new', 'brand_new'],
  ])('status %s -> %s', (status, label) => {
    expect(aiDescriptionStatusLabel(status)).toBe(label);
  });

  it('filter labels', () => {
    expect(Object.values(STATUS_FILTER_LABELS)).toEqual([
      'Черновики',
      'Одобренные',
      'Отклонённые',
      'ИИ не знает',
      'Ошибки',
    ]);
  });

  it.each(['model_refusal', 'model_truncated', 'invalid_output'])('error code %s has a readable message', (code) => {
    expect(aiDescriptionErrorLabel(code)).not.toBe(code);
    expect(aiDescriptionErrorLabel(code)).toMatch(/[А-Яа-я]/);
  });

  it('shows an unknown error code raw', () => {
    expect(aiDescriptionErrorLabel('x')).toBe('x');
  });

  it('validates URL statuses', () => {
    expect(isAIDescriptionStatus('approved')).toBe(true);
    expect(isAIDescriptionStatus('bogus')).toBe(false);
    expect(isAIDescriptionStatus(null)).toBe(false);
  });
});
