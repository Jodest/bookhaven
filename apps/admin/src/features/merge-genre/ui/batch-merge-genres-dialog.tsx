'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AdminGenre, AdminGenreMergeBatchResult, ApiError } from '@bookhaven/api-client';
import { Button, Input } from '@bookhaven/ui';
import { COLLECTIONS_QUERY_KEY } from '@/entities/curated-collection';
import { DASHBOARD_QUERY_KEY } from '@/entities/dashboard';
import { adminGenresApi, GenreStatusBadge } from '@/entities/genre';
import { MergePreviewSummary } from './merge-preview-summary';

interface BatchMergeGenresDialogProps {
  // The selected queue rows, in queue order. All of them are merged except
  // the one chosen to stay.
  genres: AdminGenre[];
  onClose: () => void;
  // Called once the merge succeeds, with the result and the target's name.
  onMerged: (result: AdminGenreMergeBatchResult, targetName: string) => void;
}

// Value of the radio that switches the target to a genre outside the
// selection (every genre id is a UUID, so it never clashes).
const OTHER_TARGET = 'other';

// Texts shared by a failed merge and a failed preview.
function batchMergeErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 400) {
      return 'Некорректный выбор жанров для слияния.';
    }
    if (err.status === 404) {
      return 'Один из жанров не найден — возможно, его уже слили. Обновите список и попробуйте снова.';
    }
  }
  return 'Не удалось выполнить слияние. Попробуйте позже.';
}

// BatchMergeGenresDialog merges several selected genres into one target,
// all-or-nothing (adminGenresApi.mergeBatch). The target is either one of the
// selected genres (radio) or any other genre found via a name search. Like
// MergeGenreDialog it is irreversible, so confirm stays disabled until there
// is at least one source and the preview has loaded.
export function BatchMergeGenresDialog({ genres, onClose, onMerged }: BatchMergeGenresDialogProps) {
  const queryClient = useQueryClient();
  const [choice, setChoice] = useState<string | null>(null);
  const [other, setOther] = useState<AdminGenre | null>(null);
  const [search, setSearch] = useState('');
  const [previewReady, setPreviewReady] = useState(false);

  const trimmedSearch = search.trim();
  const searching = choice === OTHER_TARGET && other === null;

  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'genres', 'merge-candidates', trimmedSearch],
    queryFn: () => adminGenresApi.list({ search: trimmedSearch, limit: 20 }),
    enabled: searching && trimmedSearch.length > 0,
  });

  // A selected genre can never be offered as the "other" target.
  const selectedIds = new Set(genres.map((g) => g.id));
  const candidates = (data?.items ?? []).filter((g) => !selectedIds.has(g.id));

  const target = choice === OTHER_TARGET ? other : (genres.find((g) => g.id === choice) ?? null);
  const sources = target ? genres.filter((g) => g.id !== target.id) : [];

  const mergeMutation = useMutation({
    mutationFn: () => {
      if (!target || sources.length === 0) {
        return Promise.reject(new Error('no target genre selected'));
      }
      return adminGenresApi.mergeBatch(
        sources.map((g) => g.id),
        target.id
      );
    },
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'genres'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
      queryClient.invalidateQueries({ queryKey: DASHBOARD_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: COLLECTIONS_QUERY_KEY });
      onMerged(result, target?.name ?? result.genre.name);
    },
  });

  const canConfirm = target !== null && sources.length > 0 && previewReady && !mergeMutation.isPending;
  const errorMessage = mergeMutation.error ? batchMergeErrorMessage(mergeMutation.error) : null;
  const targetName = target ? `«${target.name}»` : 'выбранный жанр';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Слияние жанров"
        className="relative z-10 max-h-full w-full max-w-lg space-y-4 overflow-y-auto rounded-lg border bg-background p-6 shadow-lg"
      >
        <h2 className="text-lg font-semibold">Слить жанры ({genres.length})</h2>

        <fieldset className="space-y-2">
          <legend className="text-sm text-muted-foreground">Какой жанр остаётся (получит книги):</legend>
          {genres.map((g) => (
            <label key={g.id} className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="merge-target"
                value={g.id}
                checked={choice === g.id}
                onChange={() => setChoice(g.id)}
              />
              <span className="font-medium">{g.name}</span>
              <GenreStatusBadge status={g.status} />
            </label>
          ))}
          <label className="flex items-center gap-2 text-sm">
            <input
              type="radio"
              name="merge-target"
              value={OTHER_TARGET}
              checked={choice === OTHER_TARGET}
              onChange={() => setChoice(OTHER_TARGET)}
            />
            <span>Другой жанр…</span>
          </label>
        </fieldset>

        {choice === OTHER_TARGET && (
          <div className="space-y-2">
            {other ? (
              <div className="flex items-center justify-between rounded-md border p-2">
                <span className="flex items-center gap-2">
                  <span className="font-medium">{other.name}</span>
                  <GenreStatusBadge status={other.status} />
                </span>
                <Button type="button" variant="ghost" size="sm" onClick={() => setOther(null)}>
                  Изменить
                </Button>
              </div>
            ) : (
              <>
                <Input
                  aria-label="Поиск целевого жанра"
                  placeholder="Поиск жанра по названию"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
                {trimmedSearch.length > 0 && (
                  <div className="max-h-48 overflow-y-auto rounded-md border">
                    {isLoading ? (
                      <div className="p-2 text-sm text-muted-foreground">Поиск...</div>
                    ) : candidates.length === 0 ? (
                      <div className="p-2 text-sm text-muted-foreground">Жанры не найдены.</div>
                    ) : (
                      candidates.map((g) => (
                        <button
                          key={g.id}
                          type="button"
                          aria-label={g.name}
                          className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent"
                          onClick={() => setOther(g)}
                        >
                          <span>{g.name}</span>
                          <GenreStatusBadge status={g.status} />
                        </button>
                      ))
                    )}
                  </div>
                )}
              </>
            )}
          </div>
        )}

        <p className="text-sm text-destructive">
          Книги выбранных жанров будут перенесены в {targetName}. Слитые жанры исчезнут из списков,
          но их названия и QID продолжат указывать на {targetName} при будущих импортах. Действие
          необратимо.
        </p>

        {target?.status === 'rejected' && (
          <div className="rounded-md bg-amber-100 p-3 text-sm text-amber-900">
            Целевой жанр отклонён — книги окажутся в скрытом жанре.
          </div>
        )}

        {target && sources.length === 0 && (
          <div className="text-sm text-muted-foreground">
            Выберите в очереди ещё хотя бы один жанр или другой целевой жанр.
          </div>
        )}

        {target && sources.length > 0 && (
          <MergePreviewSummary
            sourceIds={sources.map((g) => g.id)}
            target={target}
            onReadyChange={setPreviewReady}
            errorMessage={batchMergeErrorMessage}
          />
        )}

        {errorMessage && (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{errorMessage}</div>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={mergeMutation.isPending}>
            Отмена
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={!canConfirm}
            onClick={() => mergeMutation.mutate()}
          >
            {mergeMutation.isPending
              ? 'Слияние...'
              : `Слить ${sources.length} в ${target ? `«${target.name}»` : '…'}`}
          </Button>
        </div>
      </div>
    </div>
  );
}
