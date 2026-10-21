import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ImgHTMLAttributes } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import type { Book, CuratedCollectionDetail } from '@bookhaven/api-client';
import { CollectionDetailView } from './collection-detail-view';

const getMock = vi.fn();
const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  useSearchParams: () => searchParamsValue,
}));

vi.mock('@/entities/curated-collection/api', () => ({
  collectionsApi: { list: vi.fn(), get: (...a: unknown[]) => getMock(...a) },
}));

vi.mock('next/image', () => ({
  default: ({ fill: _fill, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean }) => (
    // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
    <img {...props} />
  ),
}));

function book(id: string, overrides: Partial<Book> = {}): Book {
  return {
    id,
    title: `Книга ${id}`,
    slug: `kniga-${id}`,
    description: null,
    coverUrl: null,
    isbn: null,
    publishedYear: null,
    pageCount: null,
    language: '',
    rating: 4.25,
    ratingsCount: 120,
    reviewsCount: 0,
    authors: [{ id: `a-${id}`, name: 'Лев Толстой', slug: 'lev-tolstoy' } as never],
    genres: [],
    createdAt: '',
    updatedAt: '',
    ...overrides,
  };
}

function detail(overrides: Partial<CuratedCollectionDetail> = {}): CuratedCollectionDetail {
  return {
    collection: {
      id: 'c1',
      slug: 'top',
      title: 'Топ-100',
      description: 'Лучшие книги',
      kind: 'manual',
      size: 100,
      shown_count: 100,
      source: 'по версии BBC',
      source_url: 'https://bbc.co.uk/bigread',
      genre: { id: 'g1', name: 'Роман', slug: 'roman' },
      cover_urls: [],
      rule: null,
      computed_at: null,
    },
    items: [
      { place: 51, movement: null, adaptation: null, book: book('1') },
      { place: 52, movement: null, adaptation: null, book: book('2', { rating: 3, ratingsCount: 0 }) },
    ],
    total: 100,
    page: 2,
    limit: 50,
    ...overrides,
  };
}

function renderView() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <CollectionDetailView slug="top" />
    </QueryClientProvider>
  );
}

describe('CollectionDetailView (/top/{slug})', () => {
  beforeEach(() => {
    getMock.mockReset();
    pushMock.mockReset();
    searchParamsValue = new URLSearchParams();
  });

  it('shows places from the API, rating, title link and the linked source', async () => {
    searchParamsValue = new URLSearchParams('page=2');
    getMock.mockResolvedValue(detail());
    renderView();

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Топ-100' })).toBeInTheDocument());
    expect(getMock).toHaveBeenCalledWith('top', { page: 2, limit: 50 });
    expect(screen.getByLabelText('Место 51')).toHaveTextContent('51');
    expect(screen.getByLabelText('Место 52')).toHaveTextContent('52');
    expect(screen.getByRole('link', { name: 'Книга 1' })).toHaveAttribute('href', '/books/kniga-1');
    expect(screen.getByText('★ 4.3 (120)')).toBeInTheDocument();
    expect(screen.getByText('Нет оценок')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Источник: по версии BBC' })).toHaveAttribute(
      'href',
      'https://bbc.co.uk/bigread'
    );
    expect(screen.getByRole('link', { name: 'Роман' })).toHaveAttribute('href', '/books?genre_id=g1');
  });

  it('source without a URL is plain text', async () => {
    getMock.mockResolvedValue(detail({ collection: { ...detail().collection, source_url: null } }));
    renderView();
    await waitFor(() => expect(screen.getByText('Источник: по версии BBC')).toBeInTheDocument());
    expect(screen.queryByRole('link', { name: 'Источник: по версии BBC' })).not.toBeInTheDocument();
  });

  it('does not link an unpublished author', async () => {
    getMock.mockResolvedValue(
      detail({
        items: [
          {
            place: 1,
            movement: null,
            adaptation: null,
            book: book('1', { authors: [{ id: 'a', name: 'Аноним', slug: null } as never] }),
          },
        ],
        total: 1,
      })
    );
    renderView();
    await waitFor(() => expect(screen.getByText('Аноним')).toBeInTheDocument());
    expect(screen.queryByRole('link', { name: 'Аноним' })).not.toBeInTheDocument();
  });

  it('keeps the page in the URL when paging', async () => {
    getMock.mockResolvedValue(detail());
    renderView();
    await waitFor(() => expect(screen.getByText('Страница 1 из 2')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Вперёд' }));
    expect(pushMock).toHaveBeenCalledWith('/top/top?page=2');
  });

  it('page 1 uses the bare URL', async () => {
    searchParamsValue = new URLSearchParams('page=2');
    getMock.mockResolvedValue(detail());
    renderView();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Назад' })).toBeEnabled());
    fireEvent.click(screen.getByRole('button', { name: 'Назад' }));
    expect(pushMock).toHaveBeenCalledWith('/top/top');
  });

  describe('automatic collections', () => {
    function autoDetail(): CuratedCollectionDetail {
      const base = detail();
      return detail({
        collection: {
          ...base.collection,
          kind: 'auto',
          source: null,
          source_url: null,
          rule: { metric: 'trending', period_days: 30, min_ratings: null, confidence: null },
          computed_at: '2026-10-14T01:00:00Z',
        },
        items: [
          { place: 1, movement: { direction: 'up', by: 3 }, adaptation: null, book: book('1') },
          { place: 2, movement: { direction: 'down', by: 2 }, adaptation: null, book: book('2') },
          { place: 3, movement: { direction: 'new', by: 0 }, adaptation: null, book: book('3') },
          { place: 4, movement: { direction: 'same', by: 0 }, adaptation: null, book: book('4') },
        ],
        total: 4,
        page: 1,
      });
    }

    it('shows movement badges for up, down and new, nothing for same', async () => {
      getMock.mockResolvedValue(autoDetail());
      renderView();
      await waitFor(() => expect(screen.getByText('↑3')).toBeInTheDocument());
      expect(screen.getByText('↓2')).toBeInTheDocument();
      expect(screen.getByText('Новинка')).toBeInTheDocument();
      expect(screen.getByLabelText('Поднялась на 3')).toBeInTheDocument();
      expect(screen.getByLabelText('Опустилась на 2')).toBeInTheDocument();
      expect(screen.getByLabelText('Новинка в подборке')).toBeInTheDocument();
      expect(screen.getAllByText(/^(↑|↓)\d+$|^Новинка$/)).toHaveLength(3);
    });

    it('shows the automatic caption', async () => {
      getMock.mockResolvedValue(autoDetail());
      renderView();
      await waitFor(() =>
        expect(
          screen.getByText(
            'Составлена автоматически по активности читателей за последние 30 дн. Обновлено 14 октября 2026.'
          )
        ).toBeInTheDocument()
      );
    });

    it('shows the source adaptation line only on verdict places that have one', async () => {
      const base = autoDetail();
      const adaptation = { id: 'w1', title: 'Война и мир', year: 1967, kind: 'film', votes_count: 40, share: 0.9 };
      getMock.mockResolvedValue({
        ...base,
        collection: {
          ...base.collection,
          rule: { metric: 'book_beats_adaptation', period_days: null, min_ratings: 5, confidence: 10 },
        },
        items: [
          { place: 1, movement: null, adaptation, book: book('1') },
          { place: 2, movement: null, adaptation: null, book: book('2') },
        ],
        total: 2,
      });
      renderView();
      await waitFor(() =>
        expect(screen.getByText('Книга лучше экранизации «Война и мир» (1967): 90% · 40 голосов')).toBeInTheDocument()
      );
      expect(screen.getAllByText(/Книга лучше экранизации/)).toHaveLength(1);
    });

    it('a weighted_rating place has no adaptation line', async () => {
      const base = autoDetail();
      getMock.mockResolvedValue({
        ...base,
        collection: {
          ...base.collection,
          rule: { metric: 'weighted_rating', period_days: null, min_ratings: 3, confidence: 10 },
        },
      });
      renderView();
      await waitFor(() => expect(screen.getByRole('heading', { name: 'Топ-100' })).toBeInTheDocument());
      expect(screen.queryByText(/экранизации/)).not.toBeInTheDocument();
    });

    it('a manual collection has no caption and no badges', async () => {
      getMock.mockResolvedValue(detail());
      renderView();
      await waitFor(() => expect(screen.getByRole('heading', { name: 'Топ-100' })).toBeInTheDocument());
      expect(screen.queryByText(/Составлена автоматически/)).not.toBeInTheDocument();
      expect(screen.queryByText(/^(↑|↓)\d+$|^Новинка$/)).not.toBeInTheDocument();
    });
  });

  it('shows «Подборка не найдена» for a 404', async () => {
    getMock.mockRejectedValue(new ApiError(404, 'Not Found', 'collection not found'));
    renderView();
    await waitFor(() => expect(screen.getByText('Подборка не найдена')).toBeInTheDocument());
  });
});
