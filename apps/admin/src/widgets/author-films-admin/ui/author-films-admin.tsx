'use client';

import { useState } from 'react';
import Link from 'next/link';
import { usePathname, useSearchParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { AdminAuthorFilm, AdminAuthorFilmSyncReport, AuthorFilmRelation } from '@bookhaven/api-client';
import { Badge, Button, Select } from '@bookhaven/ui';
import { adminAuthorsApi } from '@/entities/author';
import { kindLabel } from '@/entities/screen-work';
import { LinkScreenWorkDialog } from '@/features/link-screen-work';
import { syncErrorMessage, syncItemText, useSyncAuthorFilms } from '@/features/sync-author-films';
import { buildEditHref } from '@/shared/lib';

// «Фильмы об авторе» section of the admin author edit page (openspec
// screen-works/author-films): every linked film including hidden ones,
// hide/show, relation switch, edit and (manual links) delete per link,
// manual add and the Wikidata sync for QID authors.

const RELATION_OPTIONS = [
  { value: 'biopic', label: 'Биографический' },
  { value: 'documentary', label: 'Документальный' },
];

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
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [banner, setBanner] = useState<Banner | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const syncMutation = useSyncAuthorFilms();

  const queryKey = ['admin', 'authors', authorId, 'screen-works'];
  const { data, isLoading, isError } = useQuery({
    queryKey,
    queryFn: () => adminAuthorsApi.screenWorks(authorId),
  });

  const patchMutation = useMutation({
    mutationFn: ({ workId, patch }: { workId: string; patch: { hidden?: boolean; relation?: AuthorFilmRelation } }) =>
      patch.relation === undefined && patch.hidden !== undefined
        ? adminAuthorsApi.setScreenWorkHidden(authorId, workId, patch.hidden)
        : adminAuthorsApi.updateScreenWork(authorId, workId, patch),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (workId: string) => adminAuthorsApi.deleteScreenWork(authorId, workId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
      queryClient.invalidateQueries({ queryKey: ['admin', 'screen-works'] });
    },
  });

  const handleToggle = (item: AdminAuthorFilm) => {
    setBanner(null);
    patchMutation.mutate(
      { workId: item.id, patch: { hidden: !item.hidden } },
      {
        onError: () => setBanner({ type: 'error', message: 'Не удалось изменить видимость фильма.' }),
      }
    );
  };

  const handleRelation = (item: AdminAuthorFilm, relation: AuthorFilmRelation) => {
    setBanner(null);
    patchMutation.mutate(
      { workId: item.id, patch: { relation } },
      {
        onError: () => setBanner({ type: 'error', message: 'Не удалось изменить тип связи.' }),
      }
    );
  };

  const handleDelete = (item: AdminAuthorFilm) => {
    setBanner(null);
    if (!confirm(`Удалить связь с «${item.title}»? Сам фильм останется в каталоге.`)) return;
    deleteMutation.mutate(item.id, {
      onError: () => setBanner({ type: 'error', message: 'Не удалось удалить связь.' }),
    });
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
  const busy = patchMutation.isPending || deleteMutation.isPending;

  return (
    <section className="space-y-3 rounded-lg border p-4" aria-labelledby="author-films-admin-title">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id="author-films-admin-title" className="text-lg font-semibold">
          Фильмы об авторе
        </h2>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={() => setDialogOpen(true)}>
            Добавить фильм об авторе
          </Button>
          {hasQid && (
            <Button variant="outline" size="sm" disabled={syncMutation.isPending} onClick={handleSync}>
              Найти фильмы об авторе в Wikidata
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
                  {item.source === 'manual' && <Badge variant="secondary">Вручную</Badge>}
                </div>
                <div className="text-sm text-muted-foreground">
                  {[item.year ? String(item.year) : null, kindLabel(item.kind), item.director]
                    .filter(Boolean)
                    .join(' · ')}
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-1">
                <Select
                  aria-label="Тип связи"
                  options={RELATION_OPTIONS}
                  value={item.relation}
                  disabled={busy}
                  onChange={(e) => handleRelation(item, e.target.value as AuthorFilmRelation)}
                  className="h-8 w-44"
                />
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
          target={{ type: 'author', id: authorId, linkedIds: items.map((i) => i.id) }}
          onClose={() => setDialogOpen(false)}
          onAdded={(message) => setBanner({ type: 'success', message })}
        />
      )}
    </section>
  );
}
