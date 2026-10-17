'use client';

import { Button } from '@bookhaven/ui';
import {
  DISMISSED_MESSAGE,
  DISMISS_CONFIRM_TEXT,
  bulkDismissConfirmText,
  bulkDismissMessage,
  dismissErrorMessage,
} from '../lib/messages';
import { useBulkDismissSuggestions, useDismissSuggestion } from '../model/use-suggestion-mutations';
import type { SuggestionFeedback } from '../model/types';

interface DismissButtonProps {
  suggestionId: string;
  notify: (feedback: SuggestionFeedback | null) => void;
  disabled?: boolean;
}

/** «Отклонить» on one row: dismissing is remembered, no genre changes. */
export function DismissSuggestionButton({ suggestionId, notify, disabled }: DismissButtonProps) {
  const mutation = useDismissSuggestion();

  const dismiss = () => {
    notify(null);
    if (!window.confirm(DISMISS_CONFIRM_TEXT)) return;
    mutation.mutate(suggestionId, {
      onSuccess: () => notify({ type: 'success', message: DISMISSED_MESSAGE }),
      onError: (err) => notify({ type: 'error', message: dismissErrorMessage(err) }),
    });
  };

  return (
    <Button variant="ghost" size="sm" disabled={disabled || mutation.isPending} onClick={dismiss}>
      Отклонить
    </Button>
  );
}

interface BulkDismissButtonProps {
  suggestionIds: string[];
  notify: (feedback: SuggestionFeedback | null) => void;
  onDone: () => void;
  disabled?: boolean;
}

/** «Отклонить выбранные (N)». */
export function BulkDismissButton({ suggestionIds, notify, onDone, disabled }: BulkDismissButtonProps) {
  const mutation = useBulkDismissSuggestions();

  const dismiss = () => {
    notify(null);
    if (!window.confirm(bulkDismissConfirmText(suggestionIds.length))) return;
    mutation.mutate(suggestionIds, {
      onSuccess: (report) => {
        onDone();
        notify({ type: 'success', message: bulkDismissMessage(report) });
      },
      onError: (err) => notify({ type: 'error', message: dismissErrorMessage(err) }),
    });
  };

  return (
    <Button size="sm" variant="outline" disabled={disabled || mutation.isPending} onClick={dismiss}>
      Отклонить выбранные ({suggestionIds.length})
    </Button>
  );
}
