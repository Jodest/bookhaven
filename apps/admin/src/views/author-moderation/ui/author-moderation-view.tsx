'use client';

import { useState } from 'react';
import {
  AdminAuthor,
  AdminAuthorEnrichmentReport,
  AdminAuthorFilmSyncReport,
  AdminAuthorHint,
  AdminAuthorQueueItem,
  AdminAuthorQueueParams,
  AdminAuthorStatus,
} from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { adminAuthorsApi, AuthorStatusBadge } from '@/entities/author';
import { SampleBooksCell } from '@/entities/book';
import {
  bulkEnrichSummary,
  enrichErrorMessage,
  enrichItemText,
  enrichReportHasErrors,
  useEnrichAuthors,
} from '@/features/enrich-author';
import {
  bulkSyncSummary as bulkFilmSyncSummary,
  syncErrorMessage as filmSyncErrorMessage,
  syncItemText as filmSyncItemText,
  syncReportHasErrors as filmSyncReportHasErrors,
  useSyncAuthorFilms,
} from '@/features/sync-author-films';
import { MergeAuthorDialog } from '@/features/merge-author';
import {
  authorStatusConfirmText,
  bulkAuthorStatusConfirmText,
  bulkAuthorStatusSummary,
  useAuthorStatusMutation,
} from '@/features/moderate-author';
import { WikidataLink } from '@/shared/ui';
import {
  ModerationQueue,
  moderationCreatedAtColumn,
  type ModerationQueueColumn,
  type ModerationQueueControls,
  type ModerationQueueFilter,
  type ModerationQueueState,
  type ModerationRowContext,
} from '@/widgets/moderation-queue';

// Author moderation queue (docs/features/author-moderation-queue.md), rendered
// through the shared widgets/moderation-queue shell.

const FILTERS: ModerationQueueFilter[] = [
  {
    param: 'qid',
    label: 'QID',
    options: [
      { value: '', label: 'QID: любой' },
      { value: 'with', label: 'С QID' },
      { value: 'without', label: 'Без QID' },
    ],
  },
  {
    param: 'books',
    label: 'Книги',
    options: [
      { value: '', label: 'Книги: любые' },
      { value: 'none', label: 'Без книг' },
    ],
  },
];

const MATCH_LABELS: Record<AdminAuthorHint['match'], string> = {
  exact_name: 'то же имя',
  surname_initial: 'фамилия и инициал',
  surname: 'фамилия',
};

const SUCCESS_MESSAGES: Record<AdminAuthorStatus, (name: string) => string> = {
  published: (name) => `Автор «${name}» опубликован.`,
  rejected: (name) => `Автор «${name}» отклонён.`,
  pending: (name) => `Автор «${name}» возвращён на модерацию.`,
};

// Filter values are validated by the widget against FILTERS options.
const buildParams = ({ status, search, page, limit, filters }: ModerationQueueState): AdminAuthorQueueParams => ({
  status,
  qid: filters.qid as AdminAuthorQueueParams['qid'],
  books: filters.books as AdminAuthorQueueParams['books'],
  search,
  page,
  limit,
});

interface MergeTarget {
  duplicate: AdminAuthor;
  initialCanonical?: AdminAuthor;
}

export function AuthorModerationView() {
  const [mergeTarget, setMergeTarget] = useState<MergeTarget | null>(null);
  const statusMutation = useAuthorStatusMutation();
  const enrichMutation = useEnrichAuthors();
  const enrichPending = enrichMutation.isPending;
  const filmSyncMutation = useSyncAuthorFilms();
  const filmSyncPending = filmSyncMutation.isPending;

  const openMerge = (ctx: ModerationRowContext, duplicate: AdminAuthor, initialCanonical?: AdminAuthor) => {
    ctx.notify(null);
    setMergeTarget({ duplicate, initialCanonical });
  };

  const handleRowEnrich = (item: AdminAuthorQueueItem, ctx: ModerationRowContext) => {
    ctx.notify(null);
    enrichMutation.mutate([item.author.id], {
      onSuccess: (report: AdminAuthorEnrichmentReport) => {
        const result = report.items[0];
        if (!result) return;
        ctx.notify({
          type: result.outcome === 'error' ? 'error' : 'success',
          message: enrichItemText(result, item.author.name),
        });
      },
      onError: (err) => ctx.notify({ type: 'error', message: enrichErrorMessage(err) }),
    });
  };

  const handleBulkEnrich = (selectedItems: AdminAuthorQueueItem[], controls: ModerationQueueControls) => {
    // Only rows with a Wikidata QID are sent; the rest are reported as skipped.
    const selectedWithQid = selectedItems.filter((i) => !!i.author.external_id);
    controls.notify(null);
    if (selectedWithQid.length === 0) return;
    const skippedWithoutQid = selectedItems.length - selectedWithQid.length;
    const names = Object.fromEntries(selectedWithQid.map((i) => [i.author.id, i.author.name]));
    enrichMutation.mutate(
      selectedWithQid.map((i) => i.author.id),
      {
        onSuccess: (report: AdminAuthorEnrichmentReport) => {
          controls.clearSelection();
          controls.notify({
            type: enrichReportHasErrors(report) ? 'error' : 'success',
            message: bulkEnrichSummary(report, skippedWithoutQid, names),
          });
        },
        onError: (err) => controls.notify({ type: 'error', message: enrichErrorMessage(err) }),
      }
    );
  };

  const handleRowFilmSync = (item: AdminAuthorQueueItem, ctx: ModerationRowContext) => {
    ctx.notify(null);
    filmSyncMutation.mutate([item.author.id], {
      onSuccess: (report: AdminAuthorFilmSyncReport) => {
        const result = report.items[0];
        if (!result) return;
        ctx.notify({
          type: result.outcome === 'error' ? 'error' : 'success',
          message: filmSyncItemText(result, item.author.name),
        });
      },
      onError: (err) => ctx.notify({ type: 'error', message: filmSyncErrorMessage(err) }),
    });
  };

  const handleBulkFilmSync = (selectedItems: AdminAuthorQueueItem[], controls: ModerationQueueControls) => {
    // Only rows with a Wikidata QID are sent; the rest are reported as skipped.
    const selectedWithQid = selectedItems.filter((i) => !!i.author.external_id);
    controls.notify(null);
    if (selectedWithQid.length === 0) return;
    const skippedWithoutQid = selectedItems.length - selectedWithQid.length;
    const names = Object.fromEntries(selectedWithQid.map((i) => [i.author.id, i.author.name]));
    filmSyncMutation.mutate(
      selectedWithQid.map((i) => i.author.id),
      {
        onSuccess: (report: AdminAuthorFilmSyncReport) => {
          controls.clearSelection();
          controls.notify({
            type: filmSyncReportHasErrors(report) ? 'error' : 'success',
            message: bulkFilmSyncSummary(report, skippedWithoutQid, names),
          });
        },
        onError: (err) => controls.notify({ type: 'error', message: filmSyncErrorMessage(err) }),
      }
    );
  };

  const columns: ModerationQueueColumn<AdminAuthorQueueItem>[] = [
    {
      id: 'author',
      header: 'Автор',
      cellClassName: 'align-top',
      cell: ({ author }) => (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{author.name}</span>
            <AuthorStatusBadge status={author.status} />
          </div>
          <div className="text-xs text-muted-foreground">{author.slug}</div>
        </>
      ),
    },
    {
      id: 'qid',
      header: 'QID',
      cellClassName: 'align-top',
      cell: (item) => <WikidataLink qid={item.author.external_id} />,
    },
    moderationCreatedAtColumn((item: AdminAuthorQueueItem) => item.created_at),
    {
      id: 'books',
      header: 'Книги',
      cellClassName: 'align-top text-sm',
      cell: (item) => (
        <SampleBooksCell
          booksTotal={item.books_total}
          booksVisible={item.books_visible}
          sampleBooks={item.sample_books}
        />
      ),
    },
    {
      id: 'similar',
      header: 'Возможно, тот же человек',
      cellClassName: 'align-top text-sm',
      cell: (item, ctx) => (
        <AuthorHints
          item={item}
          pending={ctx.statusPending}
          onMerge={(canonical) => openMerge(ctx, item.author, canonical)}
        />
      ),
    },
  ];

  return (
    <ModerationQueue
      queryKey={['admin', 'authors', 'moderation']}
      buildParams={buildParams}
      queryFn={(params) => adminAuthorsApi.moderationQueue(params)}
      statusMutation={statusMutation}
      getId={(item) => item.author.id}
      getName={(item) => item.author.name}
      getStatus={(item) => item.author.status}
      filters={FILTERS}
      columns={columns}
      renderRowActions={(item, ctx) => (
        <>
          {item.author.status !== 'rejected' && (
            <Button variant="ghost" size="sm" disabled={ctx.statusPending} onClick={() => openMerge(ctx, item.author)}>
              Слить
            </Button>
          )}
          {item.author.external_id && (
            <Button variant="outline" size="sm" disabled={enrichPending} onClick={() => handleRowEnrich(item, ctx)}>
              Обогатить из Wikidata
            </Button>
          )}
          {item.author.external_id && (
            <Button
              variant="outline"
              size="sm"
              disabled={filmSyncPending}
              onClick={() => handleRowFilmSync(item, ctx)}
            >
              Найти фильмы об авторе в Wikidata
            </Button>
          )}
        </>
      )}
      renderBulkActions={(selectedItems, controls) => {
        const withQid = selectedItems.filter((i) => !!i.author.external_id).length;
        return (
          <>
            <Button
              size="sm"
              variant="outline"
              disabled={enrichPending || withQid === 0}
              onClick={() => handleBulkEnrich(selectedItems, controls)}
            >
              Обогатить из Wikidata ({withQid})
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={filmSyncPending || withQid === 0}
              onClick={() => handleBulkFilmSync(selectedItems, controls)}
            >
              Найти фильмы об авторе в Wikidata ({withQid})
            </Button>
          </>
        );
      }}
      renderOverlay={(controls) =>
        mergeTarget && (
          <MergeAuthorDialog
            duplicate={mergeTarget.duplicate}
            initialCanonical={mergeTarget.initialCanonical}
            onClose={() => setMergeTarget(null)}
            onMerged={(booksReassigned) => {
              const name = mergeTarget.duplicate.name;
              setMergeTarget(null);
              controls.clearSelection();
              controls.notify({
                type: 'success',
                message: `Автор «${name}» слит с каноническим автором. Перенесено книг: ${booksReassigned}.`,
              });
            }}
          />
        )
      }
      copy={{
        title: 'Модерация авторов',
        searchPlaceholder: 'Поиск по имени',
        loadError: 'Не удалось загрузить очередь авторов.',
        emptyPending: 'Нет авторов на модерации',
        emptyFiltered: 'Авторы не найдены',
        statusErrorFallback: 'Не удалось изменить статус автора.',
        rowConfirm: (next, item) => authorStatusConfirmText(next, item.author.name, item.books_visible),
        bulkConfirm: (next, items) =>
          bulkAuthorStatusConfirmText(
            next,
            items.length,
            items.reduce((sum, i) => sum + i.books_visible, 0)
          ),
        rowSuccess: (next, name) => SUCCESS_MESSAGES[next](name),
        bulkSummary: bulkAuthorStatusSummary,
      }}
    />
  );
}

interface AuthorHintsProps {
  item: AdminAuthorQueueItem;
  pending: boolean;
  onMerge: (canonical: AdminAuthor) => void;
}

function AuthorHints({ item, pending, onMerge }: AuthorHintsProps) {
  const ownQid = item.author.external_id;
  if (item.similar.length === 0) return <span className="text-muted-foreground">—</span>;
  return (
    <ul className="space-y-2">
      {item.similar.map((hint) => {
        const differentQid = !!ownQid && !!hint.author.external_id && hint.author.external_id !== ownQid;
        return (
          <li key={hint.author.id} className="space-y-1">
            <div className="flex flex-wrap items-center gap-1">
              <span className="font-medium">{hint.author.name}</span>
              <span className="text-xs text-muted-foreground">({MATCH_LABELS[hint.match]})</span>
              <AuthorStatusBadge status={hint.author.status} />
              {hint.author.external_id && <WikidataLink qid={hint.author.external_id} />}
            </div>
            {differentQid && <div className="text-xs text-amber-700">разные QID — вероятно, тёзки</div>}
            <Button type="button" variant="outline" size="sm" disabled={pending} onClick={() => onMerge(hint.author)}>
              Это он — слить
            </Button>
          </li>
        );
      })}
    </ul>
  );
}
