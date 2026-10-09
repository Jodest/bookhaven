import { ApiError } from '@bookhaven/api-client';

/**
 * Text of a genre create/update error. A 409 on the QID is extended with a
 * merged-genre hint: a taken QID may belong to a merged (hidden) genre whose
 * QID stays reserved (docs/features/genre-moderation-queue.md rule 6).
 */
export function genreMutationErrorText(err: unknown): string | undefined {
  if (!err) return undefined;
  if (err instanceof ApiError && err.status === 409 && /qid/i.test(err.message)) {
    return `${err.message} (возможно, у слитого жанра)`;
  }
  return err instanceof Error ? err.message : undefined;
}
