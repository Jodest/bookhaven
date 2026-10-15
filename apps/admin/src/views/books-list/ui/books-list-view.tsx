'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { adminBooksApi, BookStatusBadge, BOOK_STATUS_LABELS } from '@/entities/book';
import { adminAuthorsApi } from '@/entities/author';
import { adminGenresApi } from '@/entities/genre';
import { AdminAdaptationSyncReport, AdminBook } from '@bookhaven/api-client';
import { Badge, Button } from '@bookhaven/ui';
import { useBookStatusMutation } from '@/features/moderate-book';
import {
  hasWikidataQid,
  syncErrorMessage,
  syncItemText,
  useSyncAdaptations,
} from '@/features/sync-adaptations';
import {
  CatalogList,
  type CatalogListColumn,
  type CatalogListFilter,
  type CatalogListState,
  type CatalogRowContext,
} from '@/widgets/catalog-list';

// Catalog books list (docs/features/catalog-unified-lists.md), rendered
// through the shared widgets/catalog-list shell.

type BooksListParams = NonNullable<Parameters<typeof adminBooksApi.list>[0]>;

const SOURCE_OPTIONS = [
  { value: '', label: 'Все' },
  { value: 'manual', label: 'Добавлено вручную' },
  { value: 'harvested', label: 'Импортировано' },
];

function parseSource(value: string | undefined): 'manual' | 'harvested' | '' {
  return value === 'manual' || value === 'harvested' ? value : '';
}

// The legacy `visibility` param is never read; an invalid status means "all".
const buildParams = ({ status, search, page, limit, filters }: CatalogListState): BooksListParams => ({
  search,
  status: status ?? '',
  // Unknown author/genre ids (e.g. beyond the first 100 options) still filter.
  authorId: filters.author_id,
  genreId: filters.genre_id,
  source: parseSource(filters.source),
  page,
  limit,
});

const COLUMNS: CatalogListColumn<AdminBook>[] = [
  {
    id: 'title',
    header: 'Название',
    cell: (book) => <span className="font-medium">{book.title}</span>,
  },
  {
    id: 'authors',
    header: 'Авторы',
    cell: (book) => book.authors.map((a) => a.name).join(', ') || '—',
  },
  {
    id: 'genres',
    header: 'Жанры',
    cell: (book) => book.genres.map((g) => g.name).join(', ') || '—',
  },
  {
    id: 'status',
    header: 'Статус',
    cell: (book) => (
      <div className="flex flex-wrap gap-1">
        <BookStatusBadge status={book.status} />
        {book.external_source && <Badge variant="outline">Импортировано</Badge>}
      </div>
    ),
  },
];

export function BooksListView() {
  const statusMutation = useBookStatusMutation();
  const syncMutation = useSyncAdaptations();

  const handleSync = (book: AdminBook, ctx: CatalogRowContext) => {
    ctx.notify(null);
    syncMutation.mutate([book.id], {
      onSuccess: (report: AdminAdaptationSyncReport) => {
        const result = report.items[0];
        if (!result) return;
        ctx.notify({
          type: result.outcome === 'error' ? 'error' : 'success',
          message: syncItemText(result, book.title),
        });
      },
      onError: (err) => ctx.notify({ type: 'error', message: syncErrorMessage(err) }),
    });
  };

  const { data: authorsData } = useQuery({
    queryKey: ['admin', 'authors', 'all'],
    queryFn: () => adminAuthorsApi.list({ limit: 100 }),
  });

  const { data: genresData } = useQuery({
    queryKey: ['admin', 'genres', 'all'],
    queryFn: () => adminGenresApi.list({ limit: 100 }),
  });

  const filters: CatalogListFilter[] = [
    {
      param: 'author_id',
      label: 'Автор',
      options: [
        { value: '', label: 'Все авторы' },
        ...(authorsData?.items.map((a) => ({ value: a.id, label: a.name })) ?? []),
      ],
    },
    {
      param: 'genre_id',
      label: 'Жанр',
      options: [
        { value: '', label: 'Все жанры' },
        ...(genresData?.items.map((g) => ({ value: g.id, label: g.name })) ?? []),
      ],
    },
    { param: 'source', label: 'Источник', options: SOURCE_OPTIONS },
  ];

  return (
    <CatalogList
      title="Книги"
      headerExtra={
        <Link href={'/moderation/books' as any} className="text-sm text-primary hover:underline">
          Очередь модерации
        </Link>
      }
      entityPath="/catalog/books"
      flashKey="catalog.books"
      queryKey={['admin', 'books']}
      buildParams={buildParams}
      queryFn={(params) => adminBooksApi.list(params)}
      statusMutation={statusMutation}
      getId={(book) => book.id}
      getName={(book) => book.title}
      getStatus={(book) => book.status}
      statusLabels={BOOK_STATUS_LABELS}
      filters={filters}
      columns={COLUMNS}
      renderRowActions={(book, ctx) =>
        hasWikidataQid(book) ? (
          <Button
            variant="outline"
            size="sm"
            disabled={syncMutation.isPending}
            onClick={() => handleSync(book, ctx)}
          >
            Найти экранизации в Wikidata
          </Button>
        ) : null
      }
      copy={{
        searchPlaceholder: 'Поиск по названию, ISBN, описанию',
        loadError: 'Не удалось загрузить книги.',
        empty: 'Книги не найдены.',
        statusErrorFallback: 'Не удалось снять книгу с публикации.',
        unpublishConfirm: (name) => `Снять книгу «${name}» с публикации? Она вернётся в очередь модерации.`,
        unpublishSuccess: (name) => `Книга «${name}» снята с публикации и возвращена на модерацию.`,
      }}
    />
  );
}
