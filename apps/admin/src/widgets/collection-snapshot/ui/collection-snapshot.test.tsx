import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import type { AdminCuratedCollectionDetail, AdminCuratedCollectionItem } from '@bookhaven/api-client';
import { CollectionSnapshot } from './collection-snapshot';

const recomputeMock = vi.fn();

vi.mock('@/entities/curated-collection', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/curated-collection')>()),
  adminCollectionsApi: { recompute: (...a: unknown[]) => recomputeMock(...a) },
}));

function item(
  position: number,
  state: AdminCuratedCollectionItem['state'],
  place: number | null,
  score: number | null,
  movement: AdminCuratedCollectionItem['movement'] = null
): AdminCuratedCollectionItem {
  return {
    position,
    place,
    state,
    score,
    movement,
    book: {
      id: `b${position}`,
      title: `Книга ${position}`,
      slug: `k${position}`,
      status: 'published',
      cover_url: null,
      authors: [{ id: 'a', name: 'Автор' }],
    },
  };
}

function detail(overrides: Partial<AdminCuratedCollectionDetail> = {}): AdminCuratedCollectionDetail {
  return {
    id: 'c1',
    slug: 'top',
    title: 'Авто',
    description: '',
    kind: 'auto',
    size: 2,
    source: null,
    source_url: null,
    genre: null,
    is_published: true,
    published_at: null,
    rule: { metric: 'weighted_rating', period_days: null, min_ratings: 3, confidence: 10 },
    computed_at: '2026-10-14T01:00:00Z',
    baseline_at: null,
    created_at: '',
    updated_at: '',
    books_count: 3,
    shown_count: 2,
    items: [
      item(1, 'ranked', 1, 4.4512, { direction: 'up', by: 3 }),
      item(2, 'ranked', 2, 4.1, { direction: 'new', by: 0 }),
      item(3, 'reserve', null, 3.9, { direction: 'same', by: 0 }),
    ],
    ...overrides,
  };
}

function renderWidget(d: AdminCuratedCollectionDetail) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <CollectionSnapshot collection={d} />
    </QueryClientProvider>
  );
  return queryClient;
}

describe('CollectionSnapshot', () => {
  beforeEach(() => {
    recomputeMock.mockReset();
  });

  it('shows the snapshot with places, markers, movement and a 2-decimal weighted score', () => {
    renderWidget(detail());
    const rows = screen.getAllByRole('row').slice(1);
    expect(rows).toHaveLength(3);
    expect(within(rows[0]).getByText('№ 1')).toBeInTheDocument();
    expect(within(rows[0]).getByText('↑3')).toBeInTheDocument();
    expect(within(rows[0]).getByText('4.45')).toBeInTheDocument();
    expect(within(rows[1]).getByText('Новинка')).toBeInTheDocument();
    expect(within(rows[2]).getByText('Резерв')).toBeInTheDocument();
    expect(within(rows[2]).queryByText(/↑|↓|Новинка/)).not.toBeInTheDocument();
  });

  it('shows a whole-number score for activity metrics', () => {
    renderWidget(
      detail({
        rule: { metric: 'most_read', period_days: null, min_ratings: null, confidence: null },
        items: [item(1, 'ranked', 1, 7)],
      })
    );
    expect(screen.getByText('7')).toBeInTheDocument();
  });

  it('shows the computed time, or that it was never computed', () => {
    const { unmount } = render(
      <QueryClientProvider client={new QueryClient()}>
        <CollectionSnapshot collection={detail({ baseline_at: '2026-10-13T01:00:00Z' })} />
      </QueryClientProvider>
    );
    expect(screen.getByText(/^Пересчитано: /)).toBeInTheDocument();
    expect(screen.getByText(/^Движение — относительно пересчёта /)).toBeInTheDocument();
    unmount();

    renderWidget(detail({ computed_at: null, items: [], books_count: 0, shown_count: 0 }));
    expect(screen.getByText('Ещё не пересчитана')).toBeInTheDocument();
    expect(screen.queryByText(/Движение — относительно/)).not.toBeInTheDocument();
  });

  it('has no editing controls', () => {
    renderWidget(detail());
    expect(screen.queryByLabelText('Найти книгу в каталоге')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Вставить список' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Выше' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Убрать' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Пересчитать сейчас' })).toBeInTheDocument();
  });

  it('recompute puts the new snapshot into the detail cache', async () => {
    const fresh = detail({ computed_at: '2026-10-15T01:00:00Z', items: [item(1, 'ranked', 1, 5)] });
    recomputeMock.mockResolvedValue(fresh);
    const queryClient = renderWidget(detail());
    fireEvent.click(screen.getByRole('button', { name: 'Пересчитать сейчас' }));
    await waitFor(() => expect(recomputeMock).toHaveBeenCalledWith('c1'));
    await waitFor(() => expect(queryClient.getQueryData(['admin', 'collections', 'c1'])).toEqual(fresh));
  });

  it('shows the error of a failed recompute', async () => {
    recomputeMock.mockRejectedValue(new ApiError(422, 'Unprocessable', 'collection is not automatic'));
    renderWidget(detail());
    fireEvent.click(screen.getByRole('button', { name: 'Пересчитать сейчас' }));
    await waitFor(() =>
      expect(screen.getByText('Пересчитать можно только автоматическую подборку')).toBeInTheDocument()
    );
  });
});
