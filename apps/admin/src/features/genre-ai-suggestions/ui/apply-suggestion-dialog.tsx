'use client';

import { useState } from 'react';
import type { AdminGenre, AdminGenreAIApplyResult, AdminGenreAISuggestion } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { GenreStatusBadge, GenreTargetSearch, MergePreviewSummary } from '@/entities/genre';
import { toAdminGenre } from '../lib/genre';
import { applyErrorMessage } from '../lib/messages';
import { useApplySuggestion } from '../model/use-suggestion-mutations';

interface ApplySuggestionDialogProps {
  // An open, non-stale merge suggestion (duplicate or narrow).
  suggestion: AdminGenreAISuggestion;
  onClose: () => void;
  // Called once the merge succeeds, with the result and the used target's name.
  onApplied: (result: AdminGenreAIApplyResult, targetName: string) => void;
}

// Value of the radio that switches the target to a genre outside the
// suggestion (every genre id is a UUID, so it never clashes).
const OTHER_TARGET = 'other';

// ApplySuggestionDialog applies a merge suggestion: the suggested target is
// preselected, the admin may pick another genre of the suggestion or any
// other genre ("Другой жанр…"). When the target is inside the suggestion the
// other genres are the sources, otherwise every genre of the suggestion is.
// The merge is irreversible, so confirm stays disabled until the preview has
// loaded. The caller closes the dialog and shows the success message.
export function ApplySuggestionDialog({ suggestion, onClose, onApplied }: ApplySuggestionDialogProps) {
  const genres = suggestion.genres;
  const suggestedId = suggestion.target?.id ?? genres[0]?.id ?? '';
  const [choice, setChoice] = useState<string>(suggestedId);
  const [other, setOther] = useState<AdminGenre | null>(null);
  const [previewReady, setPreviewReady] = useState(false);
  const mutation = useApplySuggestion();

  const inGroup = genres.find((g) => g.id === choice);
  const target: AdminGenre | null =
    choice === OTHER_TARGET ? other : inGroup ? toAdminGenre(inGroup) : null;
  const sources = target ? genres.filter((g) => g.id !== target.id) : [];
  const targetName = target ? `«${target.name}»` : 'выбранный жанр';

  const canConfirm = target !== null && sources.length > 0 && previewReady && !mutation.isPending;
  const errorMessage = mutation.error ? applyErrorMessage(mutation.error) : null;

  const confirm = () => {
    if (!target) return;
    // The suggested target is the server default; only a changed one is sent.
    const targetId = target.id === suggestedId ? undefined : target.id;
    mutation.mutate(
      { id: suggestion.id, targetId },
      { onSuccess: (result) => onApplied(result, target.name) }
    );
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Применение предложения ИИ"
        className="relative z-10 max-h-full w-full max-w-lg space-y-4 overflow-y-auto rounded-lg border bg-background p-6 shadow-lg"
      >
        <h2 className="text-lg font-semibold">Применить предложение: слить жанры ({genres.length})</h2>

        <fieldset className="space-y-2">
          <legend className="text-sm text-muted-foreground">Какой жанр остаётся (получит книги):</legend>
          {genres.map((g) => (
            <label key={g.id} className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="suggestion-merge-target"
                value={g.id}
                checked={choice === g.id}
                onChange={() => setChoice(g.id)}
              />
              <span className="font-medium">{g.name}</span>
              {g.status !== '' && <GenreStatusBadge status={g.status} />}
              <span className="text-xs text-muted-foreground">книг: {g.books_total}</span>
            </label>
          ))}
          <label className="flex items-center gap-2 text-sm">
            <input
              type="radio"
              name="suggestion-merge-target"
              value={OTHER_TARGET}
              checked={choice === OTHER_TARGET}
              onChange={() => setChoice(OTHER_TARGET)}
            />
            <span>Другой жанр…</span>
          </label>
        </fieldset>

        {choice === OTHER_TARGET && (
          <div className="space-y-2">
            <GenreTargetSearch value={other} onChange={setOther} excludeIds={genres.map((g) => g.id)} />
          </div>
        )}

        <p className="text-sm text-destructive">
          Книги переносятся в {targetName}. Слитые жанры исчезнут из списков, но их названия и QID
          продолжат указывать на {targetName}. Действие необратимо.
        </p>

        {target?.status === 'rejected' && (
          <div className="rounded-md bg-amber-100 p-3 text-sm text-amber-900">
            Целевой жанр отклонён — книги окажутся в скрытом жанре.
          </div>
        )}

        {target && sources.length > 0 && (
          <MergePreviewSummary
            sourceIds={sources.map((g) => g.id)}
            target={target}
            onReadyChange={setPreviewReady}
            errorMessage={applyErrorMessage}
          />
        )}

        {errorMessage && (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{errorMessage}</div>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={mutation.isPending}>
            Отмена
          </Button>
          <Button type="button" variant="destructive" disabled={!canConfirm} onClick={confirm}>
            {mutation.isPending
              ? 'Слияние...'
              : `Слить ${sources.length} в ${target ? `«${target.name}»` : '…'}`}
          </Button>
        </div>
      </div>
    </div>
  );
}
