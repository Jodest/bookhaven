'use client';

import { useState } from 'react';
import Link from 'next/link';
import { usePathname, useSearchParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { AdminAdaptationSyncReport, AdminBookAdaptation } from '@bookhaven/api-client';
import { Badge, Button } from '@bookhaven/ui';
import { adminBooksApi } from '@/entities/book';
import { kindLabel } from '@/entities/screen-work';
import { LinkScreenWorkDialog } from '@/features/link-screen-work';
import { syncErrorMessage, syncItemText, useSyncAdaptations } from '@/features/sync-adaptations';
import { buildEditHref } from '@/shared/lib';

// «Экранизации» section of the admin book edit page (openspec
// screen-works/book-adaptations): every linked adaptation including hidden
// ones, hide/show, edit and (manual links) delete per link, manual add and
// the Wikidata sync for QID books.

interface BookAdaptationsAdminProps {
  bookId: string;
  bookTitle: string;
  /** The book has a Wikidata QID, so the sync action applies. */
  hasQid: boolean;
}

interface Banner {
  type: 'success' | 'error';
  message: string;
}

export function BookAdaptationsAdmin({ bookId, bookTitle, hasQid }: BookAdaptationsAdminProps) {
  const queryClient = useQueryClient();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [banner, setBanner] = useState<Banner | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const syncMutation = useSyncAdaptations();

  const queryKey = ['admin', 'books', bookId, 'adaptations'];
  const { data, isLoading, isError } = useQuery({
    queryKey,
    queryFn: () => adminBooksApi.adaptations(bookId),
  });

  const hideMutation = useMutation({
    mutationFn: ({ workId, hidden }: { workId: string; hidden: boolean }) =>
      adminBooksApi.setAdaptationHidden(bookId, workId, hidden),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (workId: string) => adminBooksApi.deleteAdaptation(bookId, workId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      queryClient.invalidateQueries({ queryKey: ['admin', 'screen-works'] });
    },
  });

  const handleToggle = (item: AdminBookAdaptation) => {
    setBanner(null);
    hideMutation.mutate(
      { workId: item.id, hidden: !item.hidden },
      {
        onError: () => setBanner({ type: 'error', message: 'Не удалось изменить видимость экранизации.' }),
      }
    );
  };

  const handleDelete = (item: AdminBookAdaptation) => {
    setBanner(null);
    if (!confirm(`Удалить связь с «${item.title}»? Сам фильм останется в каталоге.`)) return;
    deleteMutation.mutate(item.id, {
      onError: () => setBanner({ type: 'error', message: 'Не удалось удалить связь.' }),
    });
  };

  const handleSync = () => {
    setBanner(null);
    syncMutation.mutate([bookId], {
      onSuccess: (report: AdminAdaptationSyncReport) => {
        const result = report.items[0];
        if (!result) return;
        setBanner({
          type: result.outcome === 'error' ? 'error' : 'success',
          message: syncItemText(result, bookTitle),
        });
      },
      onError: (err) => setBanner({ type: 'error', message: syncErrorMessage(err) }),
    });
  };

  const items = data?.items ?? [];
  const busy = hideMutation.isPending || deleteMutation.isPending;

  return (
    <section className="space-y-3 rounded-lg border p-4" aria-labelledby="book-adaptations-admin-title">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id="book-adaptations-admin-title" className="text-lg font-semibold">
          Экранизации
        </h2>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={() => setDialogOpen(true)}>
            Добавить экранизацию
          </Button>
          {hasQid && (
            <Button variant="outline" size="sm" disabled={syncMutation.isPending} onClick={handleSync}>
              Найти экранизации в Wikidata
            </Button>
          )}
        </div>
      </div>

      {banner && (
        <div
          role="status"
          className={`rounded-md p-3 text-sm ${
            banner.type === 'success' ? 'bg-green-100 text-green-900' : 'bg-destructive/10 text-destructive'
          }`}
        >
          {banner.message}
        </div>
      )}

      {isLoading && <div className="h-12 animate-pulse rounded bg-muted" />}
      {isError && <p className="text-sm text-destructive">Не удалось загрузить экранизации.</p>}
      {!isLoading && !isError && items.length === 0 && (
        <p className="text-sm text-muted-foreground">Экранизаций пока нет.</p>
      )}

      {items.length > 0 && (
        <ul className="divide-y">
          {items.map((item) => (
            <li key={item.id} className="flex flex-wrap items-center justify-between gap-2 py-2">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium">{item.title}</span>
                  {item.hidden && <Badge variant="outline">Скрыта</Badge>}
                  {item.source === 'manual' && <Badge variant="secondary">Вручную</Badge>}
                </div>
                <div className="text-sm text-muted-foreground">
                  {[item.year ? String(item.year) : null, kindLabel(item.kind), item.director]
                    .filter(Boolean)
                    .join(' · ')}
                </div>
              </div>
              <div className="flex flex-wrap gap-1">
                <Button variant="ghost" size="sm" disabled={busy} onClick={() => handleToggle(item)}>
                  {item.hidden ? 'Показать' : 'Скрыть'}
                </Button>
                <Button asChild variant="ghost" size="sm">
                  <Link
                    href={
                      buildEditHref(
                        `/catalog/screen-works/${encodeURIComponent(item.id)}/edit`,
                        pathname,
                        searchParams
                      ) as any
                    }
                  >
                    Изменить
                  </Link>
                </Button>
                {item.source === 'manual' && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-destructive"
                    disabled={busy}
                    onClick={() => handleDelete(item)}
                  >
                    Удалить
                  </Button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}

      {dialogOpen && (
        <LinkScreenWorkDialog
          target={{ type: 'book', id: bookId, linkedIds: items.map((i) => i.id) }}
          onClose={() => setDialogOpen(false)}
          onAdded={(message) => setBanner({ type: 'success', message })}
        />
      )}
    </section>
  );
}
