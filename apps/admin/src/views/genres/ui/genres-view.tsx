'use client';

import Link from 'next/link';
import { adminGenresApi, GenreStatusBadge, GENRE_STATUS_LABELS } from '@/entities/genre';
import { AdminGenre } from '@bookhaven/api-client';
import { useGenreStatusMutation } from '@/features/moderate-genre';
import { WikidataLink } from '@/shared/ui';
import { CatalogList, type CatalogListColumn, type CatalogListState } from '@/widgets/catalog-list';

// Catalog genres list (docs/features/catalog-unified-lists.md), rendered
// through the shared widgets/catalog-list shell. Editing lives on
// /catalog/genres/[id]/edit.

type GenresListParams = NonNullable<Parameters<typeof adminGenresApi.list>[0]>;

const buildParams = ({ status, search, page, limit }: CatalogListState): GenresListParams => ({
  search,
  status,
  page,
  limit,
});

const COLUMNS: CatalogListColumn<AdminGenre>[] = [
  { id: 'name', header: 'Название', cell: (genre) => <span className="font-medium">{genre.name}</span> },
  { id: 'slug', header: 'Slug', cell: (genre) => genre.slug },
  { id: 'status', header: 'Статус', cell: (genre) => <GenreStatusBadge status={genre.status} /> },
  { id: 'qid', header: 'QID', cell: (genre) => <WikidataLink qid={genre.wikidata_qid} /> },
];

export function GenresView() {
  const statusMutation = useGenreStatusMutation();

  return (
    <CatalogList
      title="Жанры"
      headerExtra={
        <Link href={'/moderation/genres' as any} className="text-sm text-primary hover:underline">
          Очередь модерации
        </Link>
      }
      entityPath="/catalog/genres"
      flashKey="catalog.genres"
      queryKey={['admin', 'genres']}
      buildParams={buildParams}
      queryFn={(params) => adminGenresApi.list(params)}
      statusMutation={statusMutation}
      getId={(genre) => genre.id}
      getName={(genre) => genre.name}
      getStatus={(genre) => genre.status}
      statusLabels={GENRE_STATUS_LABELS}
      columns={COLUMNS}
      copy={{
        searchPlaceholder: 'Поиск по названию',
        loadError: 'Не удалось загрузить жанры.',
        empty: 'Жанры не найдены.',
        statusErrorFallback: 'Не удалось снять жанр с публикации.',
        unpublishConfirm: (name) => `Снять жанр «${name}» с публикации? Он вернётся в очередь модерации.`,
        unpublishSuccess: (name) => `Жанр «${name}» снят с публикации и возвращён на модерацию.`,
      }}
    />
  );
}
