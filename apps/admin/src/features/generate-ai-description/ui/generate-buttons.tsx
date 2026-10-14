'use client';

import type { AIDescriptionStatus } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import {
  GENERATE_QUEUED_TEXT,
  REGENERATE_APPROVED_CONFIRM,
  generateErrorText,
  generateMissingConfirmText,
} from '../lib/generate-text';
import { useGenerateAiDescription } from '../model/use-generate';

/** Structurally the feedback banners of the panel and the review tab. */
export interface GenerateFeedback {
  type: 'success' | 'error';
  message: string;
}

interface GenerateButtonProps {
  bookId: string;
  /** Status of the book's current record, or null when it has none. */
  currentStatus: AIDescriptionStatus | null;
  notify: (feedback: GenerateFeedback | null) => void;
  /** Called after the task was queued. */
  onQueued?: () => void;
}

/**
 * «Сгенерировать» / «Сгенерировать заново» for one book. Regenerating an
 * approved description asks first: the new draft replaces it, so it leaves
 * the site until reviewed.
 */
export function GenerateButton({ bookId, currentStatus, notify, onQueued }: GenerateButtonProps) {
  const mutation = useGenerateAiDescription();

  const generate = () => {
    notify(null);
    if (currentStatus === 'approved' && !window.confirm(REGENERATE_APPROVED_CONFIRM)) return;
    mutation.mutate(
      { book_ids: [bookId] },
      {
        onSuccess: () => {
          notify({ type: 'success', message: GENERATE_QUEUED_TEXT });
          onQueued?.();
        },
        onError: (err) => notify({ type: 'error', message: generateErrorText(err) }),
      }
    );
  };

  return (
    <Button size="sm" variant="outline" disabled={mutation.isPending} onClick={generate}>
      {currentStatus === null ? 'Сгенерировать' : 'Сгенерировать заново'}
    </Button>
  );
}

interface GenerateMissingButtonProps {
  missing: number;
  maxBulk: number;
  notify: (feedback: GenerateFeedback | null) => void;
}

/** «Сгенерировать для книг без описания» (one all_missing run, up to max_bulk). */
export function GenerateMissingButton({ missing, maxBulk, notify }: GenerateMissingButtonProps) {
  const mutation = useGenerateAiDescription();

  const generate = () => {
    notify(null);
    if (!window.confirm(generateMissingConfirmText(missing, maxBulk))) return;
    mutation.mutate(
      { all_missing: true },
      {
        onSuccess: () => notify({ type: 'success', message: GENERATE_QUEUED_TEXT }),
        onError: (err) => notify({ type: 'error', message: generateErrorText(err) }),
      }
    );
  };

  return (
    <Button size="sm" variant="outline" disabled={mutation.isPending || missing === 0} onClick={generate}>
      Сгенерировать для книг без описания
    </Button>
  );
}
