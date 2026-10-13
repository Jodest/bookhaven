import { describe, expect, it } from 'vitest';
import { ApiError, type AdminBookAIVerdict, type AdminBookQueueItem } from '@bookhaven/api-client';
import { confidentPublishIds } from './confident-publish';
import { aiReviewAllConfirmText, aiReviewErrorText } from './ai-review-text';

function item(id: string, verdict: Partial<AdminBookAIVerdict> | null, hints = 0): AdminBookQueueItem {
  return {
    book: { id } as AdminBookQueueItem['book'],
    external_id: null,
    created_at: '2026-10-13T10:00:00Z',
    similar: Array.from({ length: hints }, () => ({
      book: { id: `h-${id}` } as AdminBookQueueItem['similar'][number]['book'],
      match: 'same_isbn' as const,
    })),
    ai_verdict:
      verdict === null
        ? null
        : {
            verdict: 'publish',
            reason_codes: [],
            reason: '',
            confidence: 0.95,
            model: 'claude-haiku-5-5',
            prompt_version: 1,
            created_at: '2026-10-13T10:00:00Z',
            stale: false,
            ...verdict,
          },
  };
}

describe('confidentPublishIds', () => {
  it('keeps fresh publish verdicts at or above 0.9 without hints', () => {
    const items = [
      item('ok', {}),
      item('edge', { confidence: 0.9 }),
      item('low', { confidence: 0.89 }),
      item('stale', { stale: true }),
      item('hint', {}, 1),
      item('reject', { verdict: 'reject', reason_codes: ['duplicate'] }),
      item('human', { verdict: 'needs_human', reason_codes: ['insufficient_data'] }),
      item('none', null),
    ];
    expect(confidentPublishIds(items)).toEqual(['ok', 'edge']);
  });

  it('matches the spec scenario: 5 confident, one stale, one with a hint -> 3', () => {
    const items = [item('a', {}), item('b', {}), item('c', {}), item('d', { stale: true }), item('e', {}, 1)];
    expect(confidentPublishIds(items)).toEqual(['a', 'b', 'c']);
  });
});

describe('ai review texts', () => {
  it('confirm and error texts', () => {
    expect(aiReviewAllConfirmText(7)).toBe('Отправить на проверку ИИ все непроверенные книги (7)?');
    expect(aiReviewErrorText(new ApiError(409, 'AI review already queued'))).toBe('Проверка ИИ уже в очереди.');
    expect(aiReviewErrorText(new ApiError(503, 'AI moderation is disabled'))).toBe('ИИ-проверка выключена.');
    expect(aiReviewErrorText(new Error('x'))).toBe('Не удалось поставить проверку ИИ в очередь.');
  });
});
