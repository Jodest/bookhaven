'use client';

import { useCallback, useState } from 'react';
import type { AdminGenreAIBulkApplyReport, AdminGenreAISuggestion } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { MergePreviewSummary } from '@/entities/genre';
import { suggestionActionLabel } from '@/entities/genre-ai-suggestion';
import { toAdminGenre } from '../lib/genre';
import { applyErrorMessage } from '../lib/messages';
import { useBulkApplySuggestions } from '../model/use-suggestion-mutations';

interface BulkApplyDialogProps {
  // The selected open suggestions, in table order; at most 50.
  suggestions: AdminGenreAISuggestion[];
  onClose: () => void;
  onApplied: (report: AdminGenreAIBulkApplyReport) => void;
}

// BulkApplyDialog lists every selected suggestion with its action. A merge
// suggestion shows the preview of its suggested target; the confirm button is
// enabled once every preview has loaded or failed (a failed one is shown on
// its item). The suggestions are applied with their suggested targets.
export function BulkApplyDialog({ suggestions, onClose, onApplied }: BulkApplyDialogProps) {
  const mutation = useBulkApplySuggestions();
  // Ids of the merge suggestions whose preview is not settled yet.
  const [settled, setSettled] = useState<Record<string, boolean>>({});

  const mergeIds = suggestions.filter((s) => s.kind !== 'junk').map((s) => s.id);
  const allSettled = mergeIds.every((id) => settled[id]);

  const setItemSettled = useCallback((id: string, value: boolean) => {
    setSettled((prev) => (prev[id] === value ? prev : { ...prev, [id]: value }));
  }, []);

  const canConfirm = allSettled && suggestions.length > 0 && !mutation.isPending;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Применение выбранных предложений"
        className="relative z-10 max-h-full w-full max-w-2xl space-y-4 overflow-y-auto rounded-lg border bg-background p-6 shadow-lg"
      >
        <h2 className="text-lg font-semibold">Применить выбранные предложения ({suggestions.length})</h2>
        <p className="text-sm text-destructive">
          Слияния можно будет отменить в «Истории слияний». Предложения применяются по очереди с предложенными целевыми жанрами;
          каждое слияние — отдельная транзакция.
        </p>

        <ul className="space-y-3">
          {suggestions.map((s) => (
            <BulkApplyItem key={s.id} suggestion={s} onSettledChange={setItemSettled} />
          ))}
        </ul>

        {mutation.error && (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            Не удалось применить предложения. Попробуйте позже.
          </div>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={mutation.isPending}>
            Отмена
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={!canConfirm}
            onClick={() =>
              mutation.mutate(
                suggestions.map((s) => ({ id: s.id })),
                { onSuccess: (report) => onApplied(report) }
              )
            }
          >
            {mutation.isPending ? 'Применение...' : `Применить (${suggestions.length})`}
          </Button>
        </div>
      </div>
    </div>
  );
}

interface BulkApplyItemProps {
  suggestion: AdminGenreAISuggestion;
  onSettledChange: (id: string, settled: boolean) => void;
}

function BulkApplyItem({ suggestion, onSettledChange }: BulkApplyItemProps) {
  const { id } = suggestion;
  const handleSettled = useCallback((value: boolean) => onSettledChange(id, value), [id, onSettledChange]);
  const noop = useCallback(() => {}, []);

  const target = suggestion.genres.find((g) => g.id === suggestion.target?.id);
  const sources = suggestion.genres.filter((g) => g.id !== suggestion.target?.id);

  return (
    <li className="space-y-2 rounded-md border p-3">
      <div className="text-sm font-medium">
        {suggestionActionLabel(suggestion.kind, suggestion.target?.name ?? null)}
      </div>
      <div className="text-sm text-muted-foreground">{suggestion.genres.map((g) => g.name).join(', ')}</div>
      {suggestion.kind !== 'junk' && target && (
        <MergePreviewSummary
          sourceIds={sources.map((g) => g.id)}
          target={toAdminGenre(target)}
          onReadyChange={noop}
          onSettledChange={handleSettled}
          errorMessage={applyErrorMessage}
        />
      )}
    </li>
  );
}
