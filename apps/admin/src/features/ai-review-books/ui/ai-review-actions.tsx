'use client';

import type { AdminBookQueueItem } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { confidentPublishIds } from '../lib/confident-publish';
import { AI_REVIEW_QUEUED_TEXT, aiReviewAllConfirmText, aiReviewErrorText } from '../lib/ai-review-text';
import { useAiReviewMutation } from '../model/use-ai-review';

/** Structurally the queue widget's feedback banner. */
export interface AiReviewFeedback {
  type: 'success' | 'error';
  message: string;
}

interface AiReviewToolbarProps {
  pageItems: AdminBookQueueItem[];
  /** Status endpoint: AI moderation is on. Off hides the check controls only. */
  enabled: boolean;
  needsReview: number;
  /** Replaces the queue selection (never publishes anything). */
  onSelect: (ids: string[]) => void;
  notify: (feedback: AiReviewFeedback | null) => void;
}

/**
 * Toolbar of the «На модерации» tab: «Без проверки ИИ: N» and «Проверить
 * ИИ все непроверенные» (when enabled), and «Выбрать уверенные
 * «Опубликовать» (N)» (when the page has such rows), which only selects.
 */
export function AiReviewToolbar({ pageItems, enabled, needsReview, onSelect, notify }: AiReviewToolbarProps) {
  const mutation = useAiReviewMutation();
  const confident = confidentPublishIds(pageItems);

  if (!enabled && confident.length === 0) return null;

  const queueAll = () => {
    notify(null);
    if (!confirm(aiReviewAllConfirmText(needsReview))) return;
    mutation.mutate(
      { all_unreviewed: true },
      {
        onSuccess: () => notify({ type: 'success', message: AI_REVIEW_QUEUED_TEXT }),
        onError: (err) => notify({ type: 'error', message: aiReviewErrorText(err) }),
      }
    );
  };

  return (
    <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Проверка ИИ">
      {enabled && (
        <>
          <span className="text-sm text-muted-foreground">Без проверки ИИ: {needsReview}</span>
          <Button size="sm" variant="outline" disabled={mutation.isPending} onClick={queueAll}>
            Проверить ИИ все непроверенные
          </Button>
        </>
      )}
      {confident.length > 0 && (
        <Button size="sm" variant="outline" onClick={() => onSelect(confident)}>
          Выбрать уверенные «Опубликовать» ({confident.length})
        </Button>
      )}
    </div>
  );
}

interface AiReviewSelectedButtonProps {
  selected: AdminBookQueueItem[];
  notify: (feedback: AiReviewFeedback | null) => void;
  clearSelection: () => void;
}

/** «Проверить ИИ (N)» for the selected rows (forces a fresh check). */
export function AiReviewSelectedButton({ selected, notify, clearSelection }: AiReviewSelectedButtonProps) {
  const mutation = useAiReviewMutation();
  if (selected.length === 0) return null;

  const queueSelected = () => {
    notify(null);
    mutation.mutate(
      { book_ids: selected.map((item) => item.book.id) },
      {
        onSuccess: () => {
          clearSelection();
          notify({ type: 'success', message: AI_REVIEW_QUEUED_TEXT });
        },
        onError: (err) => notify({ type: 'error', message: aiReviewErrorText(err) }),
      }
    );
  };

  return (
    <Button size="sm" variant="outline" disabled={mutation.isPending} onClick={queueSelected}>
      Проверить ИИ ({selected.length})
    </Button>
  );
}
