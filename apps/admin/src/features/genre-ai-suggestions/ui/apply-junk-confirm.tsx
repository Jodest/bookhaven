'use client';

import type { AdminGenreAIApplyResult, AdminGenreAISuggestion } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { applyErrorMessage, junkConfirmText } from '../lib/messages';
import { useApplySuggestion } from '../model/use-suggestion-mutations';

interface ApplyJunkConfirmProps {
  // An open, non-stale junk suggestion.
  suggestion: AdminGenreAISuggestion;
  onClose: () => void;
  onApplied: (result: AdminGenreAIApplyResult, genreName: string) => void;
}

// ApplyJunkConfirm asks before rejecting the genre of a junk suggestion. The
// genre keeps its book links; it only disappears from the book pages.
export function ApplyJunkConfirm({ suggestion, onClose, onApplied }: ApplyJunkConfirmProps) {
  const mutation = useApplySuggestion();
  const genre = suggestion.genres[0];
  const errorMessage = mutation.error ? applyErrorMessage(mutation.error) : null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Отклонение жанра"
        className="relative z-10 w-full max-w-md space-y-4 rounded-lg border bg-background p-6 shadow-lg"
      >
        <p className="text-sm">{junkConfirmText(genre.name, genre.books_total)}</p>

        {errorMessage && (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{errorMessage}</div>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={mutation.isPending}>
            Отмена
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={mutation.isPending}
            onClick={() =>
              mutation.mutate(
                { id: suggestion.id },
                { onSuccess: (result) => onApplied(result, genre.name) }
              )
            }
          >
            {mutation.isPending ? 'Отклонение...' : 'Отклонить жанр'}
          </Button>
        </div>
      </div>
    </div>
  );
}
