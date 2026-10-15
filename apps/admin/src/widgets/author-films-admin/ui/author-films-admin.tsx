'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { AdminAuthorFilm, AdminAuthorFilmSyncReport, ScreenWorkKind } from '@bookhaven/api-client';
import { Badge, Button } from '@bookhaven/ui';
import { adminAuthorsApi } from '@/entities/author';
import { syncErrorMessage, syncItemText, useSyncAuthorFilms } from '@/features/sync-author-films';

// «Фильмы об авторе» section of the admin author edit page (openspec
// screen-works/author-films): every linked film including hidden ones,
// hide/show per link and the Wikidata sync for QID authors.

const KIND_LABELS: Record<ScreenWorkKind, string> = {
  film: 'Фильм',
  tv_film: 'Телефильм',
  series: 'Сериал',
  animated_film: 'Мультфильм',
  animated_series: 'Мультсериал',
  teleplay: 'Телеспектакль',
};

interface AuthorFilmsAdminProps {
  authorId: string;
  authorName: string;
  /** The author has a Wikidata QID, so the sync action applies. */
  hasQid: boolean;
}

interface Banner {
  type: 'success' | 'error';
  message: string;
}

export function AuthorFilmsAdmin({ authorId, authorName, hasQid }: AuthorFilmsAdminProps) {
  const queryClient = useQueryClient();
  const [banner, setBanner] = useState<Banner | null>(null);
  const syncMutation = useSyncAuthorFilms();

  const queryKey = ['admin', 'authors', authorId, 'screen-works'];
  const { data, isLoading, isError } = useQuery({
    queryKey,
    queryFn: () => adminAuthorsApi.screenWorks(authorId),
  });

  const hideMutation = useMutation({
    mutationFn: ({ workId, hidden }: { workId: string; hidden: boolean }) =>
      adminAuthorsApi.setScreenWorkHidden(authorId, workId, hidden),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
    },
  });

  const handleToggle = (item: AdminAuthorFilm) => {
    setBanner(null);
    hideMutation.mutate(
      { workId: item.id, hidden: !item.hidden },
      {
        onError: () => setBanner({ type: 'error', message: 'Не удалось изменить видимость фильма.' }),
      }
    );
  };

  const handleSync = () => {
    setBanner(null);
    syncMutation.mutate([authorId], {
      onSuccess: (report: AdminAuthorFilmSyncReport) => {
        const result = report.items[0];
        if (!result) return;
        setBanner({
          type: result.outcome === 'error' ? 'error' : 'success',
          message: syncItemText(result, authorName),
        });
      },
      onError: (err) => setBanner({ type: 'error', message: syncErrorMessage(err) }),
    });
  };

  const items = data?.items ?? [];

  return (
    <section className="space-y-3 rounded-lg border p-4" aria-labelledby="author-films-admin-title">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id="author-films-admin-title" className="text-lg font-semibold">
          Фильмы об авторе
        </h2>
        {hasQid && (
          <Button variant="outline" size="sm" disabled={syncMutation.isPending} onClick={handleSync}>
            Найти фильмы об авторе в Wikidata
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
      {isError && <p className="text-sm text-destructive">Не удалось загрузить фильмы об авторе.</p>}
      {!isLoading && !isError && items.length === 0 && (
        <p className="text-sm text-muted-foreground">Фильмов об авторе пока нет.</p>
      )}

      {items.length > 0 && (
        <ul className="divide-y">
          {items.map((item) => (
            <li key={item.id} className="flex flex-wrap items-center justify-between gap-2 py-2">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium">{item.title}</span>
                  {item.relation === 'documentary' && <Badge variant="secondary">Документальный</Badge>}
                  {item.hidden && <Badge variant="outline">Скрыт</Badge>}
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
