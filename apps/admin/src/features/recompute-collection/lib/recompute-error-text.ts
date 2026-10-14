import { ApiError } from '@bookhaven/api-client';

/** Text of a «Пересчитать сейчас» error. */
export function recomputeErrorText(err: unknown): string | undefined {
  if (!err) return undefined;
  if (err instanceof ApiError && err.status === 422) return 'Пересчитать можно только автоматическую подборку';
  return err instanceof Error ? err.message : undefined;
}
