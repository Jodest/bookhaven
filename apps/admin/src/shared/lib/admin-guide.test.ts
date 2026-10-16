import { describe, expect, it } from 'vitest';
import { readGuideChapter } from './admin-guide';

describe('readGuideChapter', () => {
  it('reads an existing chapter', () => {
    expect(readGuideChapter('moderation').startsWith('# Модерация отзывов')).toBe(true);
  });

  it('throws for an unknown chapter', () => {
    expect(() => readGuideChapter('no-such-chapter')).toThrow(/not found/);
  });

  it('rejects path traversal', () => {
    expect(() => readGuideChapter('../README')).toThrow(/Invalid/);
  });
});
