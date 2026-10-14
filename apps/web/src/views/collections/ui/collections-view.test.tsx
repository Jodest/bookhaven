import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ImgHTMLAttributes } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CuratedCollection } from '@bookhaven/api-client';
import { CollectionsView } from './collections-view';

const listMock = vi.fn();

vi.mock('@/entities/curated-collection/api', () => ({
  collectionsApi: { list: () => listMock(), get: vi.fn() },
}));

vi.mock('next/image', () => ({
  default: ({ fill: _fill, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean }) => (
    // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
    <img {...props} />
  ),
}));

function make(overrides: Partial<CuratedCollection> = {}): CuratedCollection {
  return {
    id: 'c1',
    slug: 'top-100',
    title: 'Топ-100 книг всех времён',
    description: '',
    kind: 'manual',
    size: 100,
    shown_count: 98,
    source: 'по версии BBC Big Read, 2003',
    source_url: 'https://bbc.co.uk',
    genre: null,
    cover_urls: ['https://covers.openlibrary.org/a.jpg', 'https://covers.openlibrary.org/b.jpg'],
    ...overrides,
  };
}

function renderView() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <CollectionsView />
    </QueryClientProvider>
  );
}

describe('CollectionsView (/top)', () => {
  beforeEach(() => listMock.mockReset());

  it('shows the empty state', async () => {
    listMock.mockResolvedValue({ items: [], total: 0 });
    renderView();
    await waitFor(() => expect(screen.getByText('Подборок пока нет')).toBeInTheDocument());
  });

  it('renders cards with covers, count and source', async () => {
    listMock.mockResolvedValue({ items: [make()], total: 1 });
    renderView();
    const card = await screen.findByRole('link', { name: /Топ-100 книг всех времён/ });
    expect(card).toHaveAttribute('href', '/top/top-100');
    expect(card.querySelectorAll('img')).toHaveLength(2);
    expect(screen.getByText('98 книг')).toBeInTheDocument();
    expect(screen.getByText('Источник: по версии BBC Big Read, 2003')).toBeInTheDocument();
  });
});
