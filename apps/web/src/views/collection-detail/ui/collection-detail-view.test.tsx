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
    },
    items: [{ place: 51, book: book('1') }, { place: 52, book: book('2', { rating: 3, ratingsCount: 0 }) }],
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
        items: [{ place: 1, book: book('1', { authors: [{ id: 'a', name: 'Аноним', slug: null } as never] }) }],
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

  it('shows «Подборка не найдена» for a 404', async () => {
    getMock.mockRejectedValue(new ApiError(404, 'Not Found', 'collection not found'));
    renderView();
    await waitFor(() => expect(screen.getByText('Подборка не найдена')).toBeInTheDocument());
  });
});
