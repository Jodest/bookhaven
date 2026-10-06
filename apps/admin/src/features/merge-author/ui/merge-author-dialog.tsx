'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { adminAuthorsApi } from '@/entities/author';
import { AdminAuthor, ApiError } from '@bookhaven/api-client';
import { Button, Input } from '@bookhaven/ui';

interface MergeAuthorDialogProps {
  // The author the "Merge" action was triggered on - pre-filled as the
  // duplicate/source author that will be deleted (docs/features/
  // duplicate-author-merge.md's UX section). Read-only in this dialog.
  duplicate: AdminAuthor;
  // Optional preselected canonical author (e.g. a similar-name hint from
  // the author moderation queue). Still changeable via "Изменить".
  initialCanonical?: Pick<AdminAuthor, 'id' | 'name'>;
  onClose: () => void;
  // Called once the merge API call succeeds, with the number of books
  // reassigned - the caller (AuthorsView) uses this to show a success
  // message and to know the dialog's job is done.
  onMerged: (booksReassigned: number) => void;
}

// MergeAuthorDialog lets an admin consolidate `duplicate` into a
// canonical author picked via a name search (reusing adminAuthorsApi.list,
// the same query AuthorsView's own search box and BookForm's author
// multi-select already use). The merge is irreversible, so the confirm
// button stays disabled until a distinct canonical author is selected and
// its label names the duplicate author being deleted.
export function MergeAuthorDialog({
  duplicate,
  initialCanonical,
  onClose,
  onMerged,
}: MergeAuthorDialogProps) {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState('');
  const [canonical, setCanonical] = useState<Pick<AdminAuthor, 'id' | 'name'> | null>(
    initialCanonical ?? null
  );

  const trimmedSearch = search.trim();

  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'authors', 'merge-candidates', trimmedSearch],
    queryFn: () => adminAuthorsApi.list({ search: trimmedSearch, limit: 20 }),
    enabled: trimmedSearch.length > 0,
  });

  // The duplicate itself can never be picked as its own canonical target,
  // regardless of what the search turns up.
  const candidates = (data?.items ?? []).filter((a) => a.id !== duplicate.id);

  const mergeMutation = useMutation({
    mutationFn: () => {
      if (!canonical) {
        return Promise.reject(new Error('no canonical author selected'));
      }
      return adminAuthorsApi.merge(duplicate.id, canonical.id);
    },
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'dashboard'] });
      onMerged(result.books_reassigned);
    },
  });

  const canConfirm = canonical !== null && canonical.id !== duplicate.id && !mergeMutation.isPending;

  const errorMessage = (() => {
    const err = mergeMutation.error;
    if (!err) return null;
    if (err instanceof ApiError) {
      if (err.status === 400) {
        return 'Нельзя слить автора с самим собой.';
      }
      if (err.status === 404) {
        return 'Один из авторов не найден — возможно, уже удалён или слит кем-то другим. Обновите список и попробуйте снова.';
      }
    }
    return 'Не удалось выполнить слияние. Попробуйте позже.';
  })();

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Слияние авторов"
        className="relative z-10 w-full max-w-lg space-y-4 rounded-lg border bg-background p-6 shadow-lg"
      >
        <h2 className="text-lg font-semibold">Слить дубликат автора</h2>

        <div className="space-y-1">
          <p className="text-sm text-muted-foreground">Дубликат (будет удалён):</p>
          <p className="font-medium">{duplicate.name}</p>
        </div>

        <div className="space-y-2">
          <p className="text-sm text-muted-foreground">
            Канонический автор (останется; книги дубликата будут перенесены на него):
          </p>
          {canonical ? (
            <div className="flex items-center justify-between rounded-md border p-2">
              <span className="font-medium">{canonical.name}</span>
              <Button type="button" variant="ghost" size="sm" onClick={() => setCanonical(null)}>
                Изменить
              </Button>
            </div>
          ) : (
            <>
              <Input
                aria-label="Поиск канонического автора"
                placeholder="Поиск автора по имени"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
              {trimmedSearch.length > 0 && (
                <div className="max-h-48 overflow-y-auto rounded-md border">
                  {isLoading ? (
                    <div className="p-2 text-sm text-muted-foreground">Поиск...</div>
                  ) : candidates.length === 0 ? (
                    <div className="p-2 text-sm text-muted-foreground">Авторы не найдены.</div>
                  ) : (
                    candidates.map((a) => (
                      <button
                        key={a.id}
                        type="button"
                        className="block w-full px-3 py-2 text-left text-sm hover:bg-accent"
                        onClick={() => setCanonical(a)}
                      >
                        {a.name}
                      </button>
                    ))
                  )}
                </div>
              )}
            </>
          )}
        </div>

        <p className="text-sm text-destructive">
          Это действие необратимо: все книги автора «{duplicate.name}» будут перенесены
          {canonical ? ` автору «${canonical.name}»` : ' выбранному автору'}, после чего «
          {duplicate.name}» будет удалён без возможности восстановления.
        </p>

        {errorMessage && (
          <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            {errorMessage}
          </div>
        )}

        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={mergeMutation.isPending}
          >
            Отмена
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={!canConfirm}
            onClick={() => mergeMutation.mutate()}
          >
            {mergeMutation.isPending ? 'Слияние...' : `Слить и удалить «${duplicate.name}»`}
          </Button>
        </div>
      </div>
    </div>
  );
}
