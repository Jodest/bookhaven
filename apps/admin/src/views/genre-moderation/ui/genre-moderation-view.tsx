'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import {
  AdminGenre,
  AdminGenreHint,
  AdminGenreQueueItem,
  AdminGenreQueueParams,
  AdminGenreStatus,
} from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { adminGenresApi, GenreStatusBadge } from '@/entities/genre';
import { adminGenreAiSuggestionsApi, GENRE_AI_STATUS_QUERY_KEY } from '@/entities/genre-ai-suggestion';
import { SampleBooksCell } from '@/entities/book';
import { BatchMergeGenresDialog, MergeGenreDialog } from '@/features/merge-genre';
import {
  bulkGenreStatusConfirmText,
  bulkGenreStatusSummary,
  genreStatusConfirmText,
  useGenreStatusMutation,
} from '@/features/moderate-genre';
import { WikidataLink } from '@/shared/ui';
import {
  ModerationQueue,
  moderationCreatedAtColumn,
  type ModerationQueueColumn,
  type ModerationQueueFilter,
  type ModerationQueueState,
  type ModerationRowContext,
} from '@/widgets/moderation-queue';

// Genre moderation queue (docs/features/genre-moderation-queue.md), rendered
// through the shared widgets/moderation-queue shell.

const FILTERS: ModerationQueueFilter[] = [
  {
    param: 'qid',
    label: 'QID',
    options: [
      { value: '', label: 'Все' },
      { value: 'with', label: 'С QID' },
      { value: 'without', label: 'Без QID' },
    ],
  },
];

const MATCH_LABELS: Record<AdminGenreHint['match'], string> = {
  exact_name: 'то же название',
  same_stem: 'та же основа',
  shared_word: 'общее слово',
};

const SUCCESS_MESSAGES: Record<AdminGenreStatus, (name: string) => string> = {
  published: (name) => `Жанр «${name}» опубликован.`,
  rejected: (name) => `Жанр «${name}» отклонён.`,
  pending: (name) => `Жанр «${name}» возвращён на модерацию.`,
};

// Filter values are validated by the widget against FILTERS options.
const buildParams = ({ status, search, page, limit, filters }: ModerationQueueState): AdminGenreQueueParams => ({
  status,
  qid: filters.qid as AdminGenreQueueParams['qid'],
  search,
  page,
  limit,
});

// The open merge dialog: a single merge (from a row or a hint) or a batch
// merge of the selected rows.
type MergeOverlay =
  | { kind: 'single'; source: AdminGenre; initialCanonical?: AdminGenre }
  | { kind: 'batch'; genres: AdminGenre[] };

// " Переключено подборок: N." when a merge switched collections, else "".
const switchedSuffix = (collectionsSwitched: number) =>
  collectionsSwitched > 0 ? ` Переключено подборок: ${collectionsSwitched}.` : '';

export function GenreModerationView() {
  const [overlay, setOverlay] = useState<MergeOverlay | null>(null);
  const statusMutation = useGenreStatusMutation();
  // The entry to the AI suggestions page: shown while the analysis is on or
  // open suggestions remain (they stay reachable after it is switched off).
  const aiStatus = useQuery({
    queryKey: GENRE_AI_STATUS_QUERY_KEY,
    queryFn: () => adminGenreAiSuggestionsApi.status(),
    refetchOnWindowFocus: true,
  });
  const showAiLink = !!aiStatus.data && (aiStatus.data.enabled || aiStatus.data.open_count > 0);

  const openMerge = (ctx: ModerationRowContext, source: AdminGenre, initialCanonical?: AdminGenre) => {
    ctx.notify(null);
    setOverlay({ kind: 'single', source, initialCanonical });
  };

  const columns: ModerationQueueColumn<AdminGenreQueueItem>[] = [
    {
      id: 'genre',
      header: 'Жанр',
      cellClassName: 'align-top',
      cell: ({ genre }) => (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{genre.name}</span>
            <GenreStatusBadge status={genre.status} />
          </div>
          <div className="text-xs text-muted-foreground">{genre.slug}</div>
        </>
      ),
    },
    {
      id: 'qid',
      header: 'QID',
      cellClassName: 'align-top',
      cell: (item) => <WikidataLink qid={item.genre.wikidata_qid} />,
    },
    moderationCreatedAtColumn((item: AdminGenreQueueItem) => item.created_at),
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
      header: 'Возможно, тот же жанр',
      cellClassName: 'align-top text-sm',
      cell: (item, ctx) => (
        <GenreHints
          item={item}
          pending={ctx.statusPending}
          onMerge={(canonical) => openMerge(ctx, item.genre, canonical)}
        />
      ),
    },
  ];

  return (
    <ModerationQueue
      queryKey={['admin', 'genres', 'moderation']}
      buildParams={buildParams}
      queryFn={(params) => adminGenresApi.moderationQueue(params)}
      statusMutation={statusMutation}
      getId={(item) => item.genre.id}
      getName={(item) => item.genre.name}
      getStatus={(item) => item.genre.status}
      filters={FILTERS}
      columns={columns}
      renderRowActions={(item, ctx) => (
        <Button variant="ghost" size="sm" disabled={ctx.statusPending} onClick={() => openMerge(ctx, item.genre)}>
          Слить
        </Button>
      )}
      renderToolbar={() =>
        showAiLink && aiStatus.data ? (
          <div>
            <Link href={'/moderation/genres/ai-suggestions' as any} className="text-sm text-primary hover:underline">
              Предложения ИИ ({aiStatus.data.open_count})
            </Link>
          </div>
        ) : null
      }
      renderBulkActions={(selected, controls) => (
        <Button
          size="sm"
          variant="outline"
          disabled={statusMutation.isPending}
          onClick={() => {
            controls.notify(null);
            setOverlay({ kind: 'batch', genres: selected.map((i) => i.genre) });
          }}
        >
          Слить выбранные ({selected.length})
        </Button>
      )}
      renderOverlay={(controls) => {
        if (!overlay) return null;
        if (overlay.kind === 'batch') {
          return (
            <BatchMergeGenresDialog
              genres={overlay.genres}
              onClose={() => setOverlay(null)}
              onMerged={(result, targetName) => {
                setOverlay(null);
                controls.clearSelection();
                controls.notify({
                  type: 'success',
                  message: `Слито жанров: ${result.sources_merged} в «${targetName}». Перенесено книг: ${result.books_reassigned}.${switchedSuffix(result.collections_switched)}`,
                });
              }}
            />
          );
        }
        return (
          <MergeGenreDialog
            source={overlay.source}
            initialCanonical={overlay.initialCanonical}
            onClose={() => setOverlay(null)}
            onMerged={(booksReassigned, collectionsSwitched) => {
              const name = overlay.source.name;
              setOverlay(null);
              controls.clearSelection();
              controls.notify({
                type: 'success',
                message: `Жанр «${name}» слит. Перенесено книг: ${booksReassigned}.${switchedSuffix(collectionsSwitched)}`,
              });
            }}
          />
        );
      }}
      copy={{
        title: 'Модерация жанров',
        searchPlaceholder: 'Поиск по названию',
        loadError: 'Не удалось загрузить очередь жанров.',
        emptyPending: 'Нет жанров на модерации',
        emptyFiltered: 'Жанры не найдены',
        statusErrorFallback: 'Не удалось изменить статус жанра.',
        rowConfirm: (next, item) => genreStatusConfirmText(next, item.genre.name, item.books_visible),
        bulkConfirm: (next, items) =>
          bulkGenreStatusConfirmText(
            next,
            items.length,
            items.reduce((sum, i) => sum + i.books_visible, 0)
          ),
        rowSuccess: (next, name) => SUCCESS_MESSAGES[next](name),
        bulkSummary: bulkGenreStatusSummary,
      }}
    />
  );
}

interface GenreHintsProps {
  item: AdminGenreQueueItem;
  pending: boolean;
  onMerge: (canonical: AdminGenre) => void;
}

function GenreHints({ item, pending, onMerge }: GenreHintsProps) {
  const ownQid = item.genre.wikidata_qid;
  if (item.similar.length === 0) return <span className="text-muted-foreground">—</span>;
  return (
    <ul className="space-y-2">
      {item.similar.map((hint) => {
        const differentQid = !!ownQid && !!hint.genre.wikidata_qid && hint.genre.wikidata_qid !== ownQid;
        return (
          <li key={hint.genre.id} className="space-y-1">
            <div className="flex flex-wrap items-center gap-1">
              <span className="font-medium">{hint.genre.name}</span>
              <span className="text-xs text-muted-foreground">({MATCH_LABELS[hint.match]})</span>
              <GenreStatusBadge status={hint.genre.status} />
              {hint.genre.wikidata_qid && <WikidataLink qid={hint.genre.wikidata_qid} />}
            </div>
            {differentQid && <div className="text-xs text-amber-700">разные QID — возможно, разные понятия</div>}
            <Button type="button" variant="outline" size="sm" disabled={pending} onClick={() => onMerge(hint.genre)}>
              Это он — слить
            </Button>
          </li>
        );
      })}
    </ul>
  );
}
