import { ApiError } from '@bookhaven/api-client';
import { describe, expect, it } from 'vitest';
import { recomputeErrorText } from './recompute-error-text';

describe('recomputeErrorText', () => {
  it('maps 422 to the manual-collection message', () => {
    expect(recomputeErrorText(new ApiError(422, 'Unprocessable', 'collection is not automatic'))).toBe(
      'Пересчитать можно только автоматическую подборку'
    );
  });

  it('passes other messages through', () => {
    expect(recomputeErrorText(new ApiError(404, 'Not Found', 'collection not found'))).toBe('collection not found');
    expect(recomputeErrorText(null)).toBeUndefined();
  });
});
