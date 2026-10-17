'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { AdminGenreMergeUndoResult } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { COLLECTIONS_QUERY_KEY } from '@/entities/curated-collection';
import { DASHBOARD_QUERY_KEY } from '@/entities/dashboard';
import { adminGenresApi } from '@/entities/genre';
import { undoErrorMessage } from '../model/undo-error-message';

interface UndoGenreMergeDialogProps {
  mergeId: string;
  onClose: () => void;
  // Called once the undo succeeds, with the server's result.
  onUndone: (result: AdminGenreMergeUndoResult) => void;
}

// UndoGenreMergeDialog undoes a whole merge journal entry (every source of a
// batch). It shows the undo preview (adminGenresApi.undoMergePreview, always
// fetched fresh) and keeps the confirm button disabled until the preview has
// loaded; a blocked undo (409) is reported by the preview already.
export function UndoGenreMergeDialog({ mergeId, onClose, onUndone }: UndoGenreMergeDialogProps) {
  const queryClient = useQueryClient();

  const preview = useQuery({
    queryKey: ['admin', 'genres', 'merges', mergeId, 'undo-preview'],
    queryFn: () => adminGenresApi.undoMergePreview(mergeId),
    staleTime: 0,
    retry: false,
  });

  const undoMutation = useMutation({
    mutationFn: () => adminGenresApi.undoMerge(mergeId),
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'genres'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
      queryClient.invalidateQueries({ queryKey: DASHBOARD_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: COLLECTIONS_QUERY_KEY });
      onUndone(result);
    },
  });

  const data = preview.data;
  const targetName = data?.merge.target.name;
  const canConfirm = preview.isSuccess && !undoMutation.isPending;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Отмена слияния жанров"
        className="relative z-10 w-full max-w-lg space-y-4 rounded-lg border bg-background p-6 shadow-lg"
      >
        <h2 className="text-lg font-semibold">
          {targetName ? `Отменить слияние в «${targetName}»` : 'Отменить слияние'}
        </h2>

        <p className="text-sm text-muted-foreground">
          Исходные жанры снова станут отдельными жанрами с прежним статусом, а их книги, алиасы и
          подборки вернутся к ним. Изменения, сделанные после слияния, сохранятся.
        </p>

        {preview.isLoading && <div className="text-sm text-muted-foreground">Считаем последствия…</div>}
        {preview.error && (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            {undoErrorMessage(preview.error)}
          </div>
        )}
        {data && (
          <div className="space-y-1 rounded-md border p-3 text-sm">
            <p>Вернётся книг в исходные жанры: {data.books_returned}.</p>
            {data.books_relinked > 0 && (
              <p>
                Книг, которые снова получат исходный жанр, оставаясь в «{data.merge.target.name}»:{' '}
                {data.books_relinked}.
              </p>
            )}
            {data.collections.map((c) => (
              <p key={c.id} className="text-amber-900">
                Подборка «{c.title}» вернётся к жанру «{c.source_genre.name}».
              </p>
            ))}
          </div>
        )}

        {undoMutation.error && (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            {undoErrorMessage(undoMutation.error)}
          </div>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={undoMutation.isPending}>
            Закрыть
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={!canConfirm}
            onClick={() => undoMutation.mutate()}
          >
            {undoMutation.isPending ? 'Отмена...' : 'Отменить слияние'}
          </Button>
        </div>
      </div>
    </div>
  );
}
