'use client';

import Link from 'next/link';
import { adminAuthorsApi, AuthorStatusBadge, AUTHOR_STATUS_LABELS } from '@/entities/author';
import { AdminAuthor, AdminAuthorEnrichmentReport, AdminAuthorFilmSyncReport } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { enrichErrorMessage, enrichItemText, useEnrichAuthors } from '@/features/enrich-author';
import { useAuthorStatusMutation } from '@/features/moderate-author';
import {
  syncErrorMessage as filmSyncErrorMessage,
  syncItemText as filmSyncItemText,
  useSyncAuthorFilms,
} from '@/features/sync-author-films';
import { WikidataLink } from '@/shared/ui';
import {
  CatalogList,
  type CatalogListColumn,
  type CatalogListState,
  type CatalogRowContext,
} from '@/widgets/catalog-list';

// Catalog authors list (docs/features/catalog-unified-lists.md), rendered
// through the shared widgets/catalog-list shell. Editing lives on
// /catalog/authors/[id]/edit; merging lives in Модерация only.

type AuthorsListParams = NonNullable<Parameters<typeof adminAuthorsApi.list>[0]>;

const buildParams = ({ status, search, page, limit }: CatalogListState): AuthorsListParams => ({
  search,
  status,
  page,
  limit,
});

const COLUMNS: CatalogListColumn<AdminAuthor>[] = [
  { id: 'name', header: 'Имя', cell: (author) => <span className="font-medium">{author.name}</span> },
  { id: 'slug', header: 'Slug', cell: (author) => author.slug },
  { id: 'qid', header: 'QID', cell: (author) => <WikidataLink qid={author.external_id} /> },
  { id: 'status', header: 'Статус', cell: (author) => <AuthorStatusBadge status={author.status} /> },
];

export function AuthorsView() {
  const statusMutation = useAuthorStatusMutation();
  const enrichMutation = useEnrichAuthors();
  const filmSyncMutation = useSyncAuthorFilms();

  const handleFilmSync = (author: AdminAuthor, ctx: CatalogRowContext) => {
    ctx.notify(null);
    filmSyncMutation.mutate([author.id], {
      onSuccess: (report: AdminAuthorFilmSyncReport) => {
        const result = report.items[0];
        if (!result) return;
        ctx.notify({
          type: result.outcome === 'error' ? 'error' : 'success',
          message: filmSyncItemText(result, author.name),
        });
      },
      onError: (err) => ctx.notify({ type: 'error', message: filmSyncErrorMessage(err) }),
    });
  };

  const handleEnrich = (author: AdminAuthor, ctx: CatalogRowContext) => {
    ctx.notify(null);
    enrichMutation.mutate([author.id], {
      onSuccess: (report: AdminAuthorEnrichmentReport) => {
        const result = report.items[0];
        if (!result) return;
        ctx.notify({
          type: result.outcome === 'error' ? 'error' : 'success',
          message: enrichItemText(result, author.name),
        });
      },
      onError: (err) => ctx.notify({ type: 'error', message: enrichErrorMessage(err) }),
    });
  };

  return (
    <CatalogList
      title="Авторы"
      headerExtra={
        <Link href={'/moderation/authors' as any} className="text-sm text-primary hover:underline">
          Очередь модерации
        </Link>
      }
      entityPath="/catalog/authors"
      flashKey="catalog.authors"
      queryKey={['admin', 'authors']}
      buildParams={buildParams}
      queryFn={(params) => adminAuthorsApi.list(params)}
      statusMutation={statusMutation}
      getId={(author) => author.id}
      getName={(author) => author.name}
      getStatus={(author) => author.status}
      statusLabels={AUTHOR_STATUS_LABELS}
      columns={COLUMNS}
      renderRowActions={(author, ctx) =>
        author.external_id ? (
          <>
            <Button
              variant="outline"
              size="sm"
              disabled={enrichMutation.isPending}
              onClick={() => handleEnrich(author, ctx)}
            >
              Обогатить из Wikidata
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={filmSyncMutation.isPending}
              onClick={() => handleFilmSync(author, ctx)}
            >
              Найти фильмы об авторе в Wikidata
            </Button>
          </>
        ) : null
      }
      copy={{
        searchPlaceholder: 'Поиск по имени',
        loadError: 'Не удалось загрузить авторов.',
        empty: 'Авторы не найдены.',
        statusErrorFallback: 'Не удалось снять автора с публикации.',
        unpublishConfirm: (name) => `Снять автора «${name}» с публикации? Он вернётся в очередь модерации.`,
        unpublishSuccess: (name) => `Автор «${name}» снят с публикации и возвращён на модерацию.`,
      }}
    />
  );
}
