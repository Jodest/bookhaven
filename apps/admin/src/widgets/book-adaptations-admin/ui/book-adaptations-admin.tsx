'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { AdminAdaptationSyncReport, AdminBookAdaptation, ScreenWorkKind } from '@bookhaven/api-client';
import { Badge, Button } from '@bookhaven/ui';
import { adminBooksApi } from '@/entities/book';
import { syncErrorMessage, syncItemText, useSyncAdaptations } from '@/features/sync-adaptations';

// «Экранизации» section of the admin book edit page (openspec
// screen-works/book-adaptations): every linked adaptation including hidden
// ones, hide/show per link and the Wikidata sync for QID books.

const KIND_LABELS: Record<ScreenWorkKind, string> = {
  film: 'Фильм',
  tv_film: 'Телефильм',
  series: 'Сериал',
  animated_film: 'Мультфильм',
  animated_series: 'Мультсериал',
  teleplay: 'Телеспектакль',
};

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
  const [banner, setBanner] = useState<Banner | null>(null);
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

  const handleToggle = (item: AdminBookAdaptation) => {
    setBanner(null);
    hideMutation.mutate(
      { workId: item.id, hidden: !item.hidden },
      {
        onError: () => setBanner({ type: 'error', message: 'Не удалось изменить видимость экранизации.' }),
      }
    );
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

  return (
    <section className="space-y-3 rounded-lg border p-4" aria-labelledby="book-adaptations-admin-title">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id="book-adaptations-admin-title" className="text-lg font-semibold">
          Экранизации
        </h2>
        {hasQid && (
          <Button variant="outline" size="sm" disabled={syncMutation.isPending} onClick={handleSync}>
            Найти экранизации в Wikidata
          </Button>
        )}
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
                </div>
                <div className="text-sm text-muted-foreground">
                  {[item.year ? String(item.year) : null, KIND_LABELS[item.kind] ?? item.kind, item.director]
                    .filter(Boolean)
                    .join(' · ')}
                </div>
              </div>
              <Button
                variant="ghost"
                size="sm"
                disabled={hideMutation.isPending}
                onClick={() => handleToggle(item)}
              >
                {item.hidden ? 'Показать' : 'Скрыть'}
              </Button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
