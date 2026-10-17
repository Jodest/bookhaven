'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AdminGenre, ApiError } from '@bookhaven/api-client';
import { Button, Input } from '@bookhaven/ui';
import { COLLECTIONS_QUERY_KEY } from '@/entities/curated-collection';
import { DASHBOARD_QUERY_KEY } from '@/entities/dashboard';
import { adminGenresApi, GenreStatusBadge } from '@/entities/genre';
import { MergePreviewSummary } from './merge-preview-summary';

interface MergeGenreDialogProps {
  // The genre the "Слить" action was triggered on: it becomes an alias of
  // the target (docs/features/genre-moderation-queue.md rule 4). Read-only.
  source: AdminGenre;
  // Optional preselected target (a similar-name hint from the queue). Still
  // changeable via "Изменить".
  initialCanonical?: AdminGenre;
  onClose: () => void;
  // Called once the merge succeeds, with the number of books reassigned and
  // of curated collections switched to the target.
  onMerged: (booksReassigned: number, collectionsSwitched: number) => void;
}

// Texts shared by a failed merge and a failed preview.
function mergeErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 400) {
      return 'Нельзя слить жанр с самим собой.';
    }
    if (err.status === 404) {
      return 'Один из жанров не найден — возможно, его уже слили. Обновите список и попробуйте снова.';
    }
  }
  return 'Не удалось выполнить слияние. Попробуйте позже.';
}

// MergeGenreDialog merges `source` into a target genre picked via a name
// search (adminGenresApi.list, which never returns merged genres). Unlike
// the author merge the source is not deleted: it becomes a hidden alias
// whose name and QID keep resolving to the target on future imports. The
// merge is irreversible, so the confirm button stays disabled until a
// distinct target is selected and its preview (books to move, collections to
// switch) has loaded.
export function MergeGenreDialog({ source, initialCanonical, onClose, onMerged }: MergeGenreDialogProps) {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [canonical, setCanonical] = useState<AdminGenre | null>(initialCanonical ?? null);
  const [previewReady, setPreviewReady] = useState(false);

  const trimmedSearch = search.trim();

  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'genres', 'merge-candidates', trimmedSearch],
    queryFn: () => adminGenresApi.list({ search: trimmedSearch, limit: 20 }),
    enabled: trimmedSearch.length > 0,
  });

  // The source can never be picked as its own target.
  const candidates = (data?.items ?? []).filter((g) => g.id !== source.id);

  const mergeMutation = useMutation({
    mutationFn: () => {
      if (!canonical) {
        return Promise.reject(new Error('no target genre selected'));
      }
      return adminGenresApi.merge(source.id, canonical.id);
    },
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'genres'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
      queryClient.invalidateQueries({ queryKey: DASHBOARD_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: COLLECTIONS_QUERY_KEY });
      onMerged(result.books_reassigned, result.collections_switched);
    },
  });

  const canConfirm = canonical !== null && canonical.id !== source.id && previewReady && !mergeMutation.isPending;

  const errorMessage = mergeMutation.error ? mergeErrorMessage(mergeMutation.error) : null;

  const targetName = canonical ? `«${canonical.name}»` : 'выбранный жанр';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Слияние жанров"
        className="relative z-10 w-full max-w-lg space-y-4 rounded-lg border bg-background p-6 shadow-lg"
      >
        <h2 className="text-lg font-semibold">
          Слить жанр «{source.name}» в {canonical ? `«${canonical.name}»` : '…'}
        </h2>

        <div className="space-y-2">
          <p className="text-sm text-muted-foreground">Целевой жанр (получит книги):</p>
          {canonical ? (
            <div className="flex items-center justify-between rounded-md border p-2">
              <span className="flex items-center gap-2">
                <span className="font-medium">{canonical.name}</span>
                <GenreStatusBadge status={canonical.status} />
              </span>
              <Button type="button" variant="ghost" size="sm" onClick={() => setCanonical(null)}>
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
                        onClick={() => setCanonical(g)}
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

        <p className="text-sm text-destructive">
          Книги жанра «{source.name}» будут перенесены в {targetName}. «{source.name}» исчезнет из
          списков, но его название и QID продолжат указывать на {targetName} при будущих импортах.
          Действие необратимо.
        </p>

        {canonical?.status === 'rejected' && (
          <div className="rounded-md bg-amber-100 p-3 text-sm text-amber-900">
            Целевой жанр отклонён — книги окажутся в скрытом жанре.
          </div>
        )}

        {canonical && canonical.id !== source.id && (
          <MergePreviewSummary
            sourceIds={[source.id]}
            target={canonical}
            onReadyChange={setPreviewReady}
            errorMessage={mergeErrorMessage}
          />
        )}

        {errorMessage && (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            {errorMessage}
          </div>
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
              : `Слить «${source.name}» в ${canonical ? `«${canonical.name}»` : '…'}`}
          </Button>
        </div>
      </div>
    </div>
  );
}
