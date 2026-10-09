'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { usePathname, useSearchParams } from 'next/navigation';
import type {
  AdminBookHint,
  AdminBookQueueItem,
  AdminBookQueueParams,
  AdminBookStatus,
} from '@bookhaven/api-client';
import { AuthorStatusBadge } from '@/entities/author';
import { adminBooksApi, BookStatusBadge } from '@/entities/book';
import { GenreStatusBadge } from '@/entities/genre';
import {
  bookStatusConfirmText,
  bulkBookStatusConfirmText,
  bulkBookStatusSummary,
  useBookStatusMutation,
} from '@/features/moderate-book';
import { buildEditHref } from '@/shared/lib';
import {
  ModerationQueue,
  moderationCreatedAtColumn,
  type ModerationQueueColumn,
  type ModerationQueueFilter,
  type ModerationQueueState,
} from '@/widgets/moderation-queue';

// Book moderation queue (docs/features/book-moderation-ui.md), rendered
// through the shared widgets/moderation-queue shell.

const FILTERS: ModerationQueueFilter[] = [
  {
    param: 'source',
    label: 'Источник',
    options: [
      { value: '', label: 'Источник: любой' },
      { value: 'manual', label: 'Вручную' },
      { value: 'harvested', label: 'Харвестер' },
    ],
  },
];

const MATCH_LABELS: Record<AdminBookHint['match'], string> = {
  same_isbn: 'тот же ISBN',
  same_title_author: 'то же название и автор',
};

const SUCCESS_MESSAGES: Record<AdminBookStatus, (title: string) => string> = {
  published: (title) => `Книга «${title}» опубликована.`,
  rejected: (title) => `Книга «${title}» отклонена.`,
  pending: (title) => `Книга «${title}» возвращена на модерацию.`,
};

// Filter values are validated by the widget against FILTERS options.
const buildParams = ({ status, search, page, limit, filters }: ModerationQueueState): AdminBookQueueParams => ({
  status,
  source: filters.source as AdminBookQueueParams['source'],
  search,
  page,
  limit,
});

const Muted = () => <span className="text-muted-foreground">—</span>;

/** "ISBN · год" with only the parts that exist; null when both are empty. */
function isbnYear(isbn: string | null, year: number | null): string | null {
  const parts = [isbn, year != null ? String(year) : null].filter(Boolean);
  return parts.length > 0 ? parts.join(' · ') : null;
}

export function BookModerationView() {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const statusMutation = useBookStatusMutation();

  const editHref = (id: string) => buildEditHref(`/catalog/books/${id}/edit`, pathname, searchParams);

  const columns: ModerationQueueColumn<AdminBookQueueItem>[] = [
    {
      id: 'book',
      header: 'Книга',
      cellClassName: 'align-top',
      cell: ({ book }) => {
        const meta = isbnYear(book.isbn, book.published_year);
        return (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <Link href={editHref(book.id) as any} className="font-medium hover:underline">
                {book.title}
              </Link>
              <BookStatusBadge status={book.status} />
            </div>
            {meta && <div className="text-xs text-muted-foreground">{meta}</div>}
          </>
        );
      },
    },
    {
      id: 'authors',
      header: 'Авторы',
      cellClassName: 'align-top text-sm',
      cell: ({ book }) => (
        <BookPeopleCell items={book.authors} renderBadge={(status) => <AuthorStatusBadge status={status} />} />
      ),
    },
    {
      id: 'genres',
      header: 'Жанры',
      cellClassName: 'align-top text-sm',
      cell: ({ book }) => (
        <BookPeopleCell items={book.genres} renderBadge={(status) => <GenreStatusBadge status={status} />} />
      ),
    },
    {
      id: 'source',
      header: 'Источник',
      cellClassName: 'align-top text-sm',
      cell: (item) => <SourceCell source={item.book.external_source} externalId={item.external_id} />,
    },
    moderationCreatedAtColumn((item: AdminBookQueueItem) => item.created_at),
    {
      id: 'similar',
      header: 'Возможно, та же книга',
      cellClassName: 'align-top text-sm',
      cell: (item) => <BookHints hints={item.similar} editHref={editHref} />,
    },
  ];

  return (
    <ModerationQueue
      queryKey={['admin', 'books', 'moderation']}
      buildParams={buildParams}
      queryFn={(params) => adminBooksApi.moderationQueue(params)}
      statusMutation={statusMutation}
      getId={(item) => item.book.id}
      getName={(item) => item.book.title}
      getStatus={(item) => item.book.status}
      filters={FILTERS}
      columns={columns}
      flashKey="catalog.books"
      copy={{
        title: 'Модерация книг',
        searchPlaceholder: 'Поиск по названию или ISBN',
        loadError: 'Не удалось загрузить очередь книг.',
        emptyPending: 'Нет книг на модерации',
        emptyFiltered: 'Книги не найдены',
        statusErrorFallback: 'Не удалось изменить статус книги.',
        rowConfirm: (next, item) => bookStatusConfirmText(next, item.book.title),
        bulkConfirm: (next, items) => bulkBookStatusConfirmText(next, items.length),
        rowSuccess: (next, title) => SUCCESS_MESSAGES[next](title),
        bulkSummary: bulkBookStatusSummary,
      }}
    />
  );
}

interface BookPeopleCellProps<TStatus extends string> {
  items: { id: string; name: string; status: TStatus }[];
  renderBadge: (status: TStatus) => ReactNode;
}

/** Authors or genres: name plus a status badge when not published. */
function BookPeopleCell<TStatus extends string>({ items, renderBadge }: BookPeopleCellProps<TStatus>) {
  if (items.length === 0) return <Muted />;
  return (
    <ul className="space-y-1">
      {items.map((entry) => (
        <li key={entry.id} className="flex flex-wrap items-center gap-1">
          <span>{entry.name}</span>
          {entry.status !== 'published' && renderBadge(entry.status)}
        </li>
      ))}
    </ul>
  );
}

function SourceCell({ source, externalId }: { source: string | null; externalId: string | null }) {
  if (source == null) return <span>Вручную</span>;
  return (
    <>
      <div>{source}</div>
      {externalId && <div className="font-mono text-xs text-muted-foreground">{externalId}</div>}
    </>
  );
}

function BookHints({ hints, editHref }: { hints: AdminBookHint[]; editHref: (id: string) => string }) {
  if (hints.length === 0) return <Muted />;
  return (
    <ul className="space-y-2">
      {hints.map((hint) => {
        const meta = isbnYear(hint.book.isbn, hint.book.published_year);
        return (
          <li key={hint.book.id} className="space-y-1">
            <div className="flex flex-wrap items-center gap-1">
              <Link href={editHref(hint.book.id) as any} className="font-medium hover:underline">
                {hint.book.title}
              </Link>
              <span className="text-xs text-muted-foreground">({MATCH_LABELS[hint.match]})</span>
              <BookStatusBadge status={hint.book.status} />
            </div>
            {meta && <div className="text-xs text-muted-foreground">{meta}</div>}
          </li>
        );
      })}
    </ul>
  );
}
