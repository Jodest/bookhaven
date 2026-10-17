import { act, fireEvent, render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CatalogView } from './catalog-view';

type ListQuery = { page?: number; limit?: number };
type Call = { query: ListQuery; resolve: (v: unknown) => void; reject: (e: unknown) => void };

const calls: Call[] = [];
const push = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useSearchParams: () => searchParamsValue,
  usePathname: () => '/books',
  useRouter: () => ({ push }),
}));

vi.mock('next/link', () => ({
  default: ({
    href,
    scroll: _scroll,
    children,
    ...rest
  }: { href: string; scroll?: boolean; children?: ReactNode } & Record<string, unknown>) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

vi.mock('@/entities/book', () => ({
  booksApi: {
    list: (query: ListQuery) =>
      new Promise((resolve, reject) => {
        calls.push({ query, resolve, reject });
      }),
  },
  BookCard: ({ book }: { book: { title: string } }) => <div data-testid="book">{book.title}</div>,
}));

vi.mock('@/widgets/catalog-filters', async () => {
  const actual = await vi.importActual<typeof import('@/widgets/catalog-filters/ui/search-bar')>(
    '@/widgets/catalog-filters/ui/search-bar'
  );
  return {
    SearchBar: actual.SearchBar,
    FilterPanel: () => null,
    ActiveFilters: () => null,
    SortSelector: () => null,
  };
});

const book = (id: string) => ({ id, title: `Book ${id}` });
const pageItems = (page: number, n = 24) =>
  Array.from({ length: n }, (_, i) => book(`p${page}-${i}`));

async function resolveCall(index: number, page: number, total: number, n = 24) {
  await act(async () => {
    calls[index].resolve({ items: pageItems(page, n), total });
  });
}

function setUrl(qs: string) {
  searchParamsValue = new URLSearchParams(qs);
}

describe('CatalogView', () => {
  const scrollIntoView = vi.fn();
  let replaceState: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    calls.length = 0;
    push.mockReset();
    scrollIntoView.mockReset();
    Element.prototype.scrollIntoView = scrollIntoView;
    replaceState = vi.spyOn(window.history, 'replaceState');
    setUrl('');
  });

  afterEach(() => {
    replaceState.mockRestore();
  });

  it('shows skeletons on the first load and requests limit 24', () => {
    const { container } = render(<CatalogView />);
    expect(calls).toHaveLength(1);
    expect(calls[0].query).toMatchObject({ page: 1, limit: 24 });
    expect(container.querySelectorAll('.animate-pulse').length).toBeGreaterThan(0);
  });

  it('"Показать ещё" appends the next page, rewrites the URL and does not refetch on sync', async () => {
    const { rerender } = render(<CatalogView />);
    await resolveCall(0, 1, 60);
    expect(screen.getAllByTestId('book')).toHaveLength(24);
    expect(screen.getByLabelText('Предыдущая страница')).toHaveAttribute('aria-disabled', 'true');

    fireEvent.click(screen.getByRole('button', { name: 'Показать ещё 24' }));
    expect(calls).toHaveLength(2);
    expect(calls[1].query).toMatchObject({ page: 2, limit: 24 });
    expect(screen.getByRole('button', { name: 'Загрузка…' })).toBeDisabled();
    await resolveCall(1, 2, 60);

    expect(screen.getAllByTestId('book')).toHaveLength(48);
    expect(replaceState).toHaveBeenCalledTimes(1);
    expect(replaceState.mock.calls[0][2]).toBe('/books?page=2');
    expect(push).not.toHaveBeenCalled();

    // Next syncs useSearchParams from native history.
    setUrl('page=2');
    await act(async () => {
      rerender(<CatalogView />);
    });
    expect(calls).toHaveLength(2);
    expect(screen.getAllByTestId('book')).toHaveLength(48);
    expect(screen.getByLabelText('Страница 2', { selector: '[aria-current="page"]' })).toBeInTheDocument();
    expect(screen.getByLabelText('Предыдущая страница')).toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByLabelText('Следующая страница')).toHaveAttribute('href', '/books?page=3');
  });

  it('labels the last partial page and hides the button on the last page', async () => {
    const { rerender } = render(<CatalogView />);
    await resolveCall(0, 1, 55);
    expect(screen.getByRole('button', { name: 'Показать ещё 24' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Показать ещё 24' }));
    await resolveCall(1, 2, 55);
    expect(screen.getByRole('button', { name: 'Показать ещё 7' })).toBeInTheDocument();

    setUrl('page=3');
    await act(async () => {
      rerender(<CatalogView />);
    });
    await resolveCall(2, 3, 55, 7);
    expect(screen.queryByRole('button', { name: /Показать ещё/ })).not.toBeInTheDocument();
  });

  it('a direct visit of ?page=N shows page N only', async () => {
    setUrl('page=3');
    render(<CatalogView />);
    expect(calls[0].query).toMatchObject({ page: 3 });
    await resolveCall(0, 3, 100);
    expect(screen.getAllByTestId('book')).toHaveLength(24);
    expect(screen.getByLabelText('Страница 3', { selector: '[aria-current="page"]' })).toBeInTheDocument();
  });

  it('dims the old grid instead of showing skeletons on later fetches', async () => {
    const { container, rerender } = render(<CatalogView />);
    await resolveCall(0, 1, 100);

    setUrl('page=2');
    await act(async () => {
      rerender(<CatalogView />);
    });
    const grid = container.querySelector('[aria-busy="true"]');
    expect(grid).not.toBeNull();
    expect(grid).toHaveClass('opacity-50');
    expect(screen.getAllByTestId('book')).toHaveLength(24);
    expect(container.querySelector('.animate-pulse')).toBeNull();

    await resolveCall(1, 2, 100);
    expect(container.querySelector('[aria-busy="true"]')).toBeNull();
  });

  it('scrolls to the grid when a page link is clicked', async () => {
    render(<CatalogView />);
    await resolveCall(0, 1, 100);
    const link = screen.getByRole('link', { name: 'Страница 3' });
    expect(link).toHaveAttribute('href', '/books?page=3');
    fireEvent.click(link);
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', behavior: 'smooth' });
  });

  it('ignores stale responses', async () => {
    const { rerender } = render(<CatalogView />);
    await resolveCall(0, 1, 100);

    setUrl('page=2');
    await act(async () => {
      rerender(<CatalogView />);
    });
    setUrl('page=3');
    await act(async () => {
      rerender(<CatalogView />);
    });
    expect(calls).toHaveLength(3);

    await resolveCall(2, 3, 100);
    await resolveCall(1, 2, 100);
    expect(screen.getAllByTestId('book')[0]).toHaveTextContent('Book p3-0');
    expect(screen.getByLabelText('Страница 3', { selector: '[aria-current="page"]' })).toBeInTheDocument();
  });

  it('shows an out-of-range message with a link to the last page', async () => {
    setUrl('page=9&sort=title');
    render(<CatalogView />);
    await resolveCall(0, 9, 30, 0);
    expect(screen.getByText('На этой странице книг нет')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Перейти на последнюю страницу' })).toHaveAttribute(
      'href',
      '/books?page=2&sort=title'
    );
  });

  it('keeps already loaded books and allows a retry when loading more fails', async () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    render(<CatalogView />);
    await resolveCall(0, 1, 60);
    fireEvent.click(screen.getByRole('button', { name: 'Показать ещё 24' }));
    await act(async () => {
      calls[1].reject(new Error('boom'));
    });
    expect(screen.getByText('Не удалось загрузить книги')).toBeInTheDocument();
    expect(screen.getAllByTestId('book')).toHaveLength(24);
    expect(screen.getByRole('button', { name: 'Показать ещё 24' })).not.toBeDisabled();
    errorSpy.mockRestore();
  });

  describe('#459 regression', () => {
    beforeEach(() => vi.useFakeTimers());
    afterEach(() => vi.useRealTimers());

    it('a page-only URL change does not make SearchBar reset the page', async () => {
      setUrl('search=dune');
      const { rerender } = render(<CatalogView />);
      await resolveCall(0, 1, 100);

      setUrl('search=dune&page=2');
      await act(async () => {
        rerender(<CatalogView />);
      });
      await act(async () => {
        vi.advanceTimersByTime(1000);
      });
      expect(push).not.toHaveBeenCalled();
      expect(calls[1].query).toMatchObject({ page: 2 });
    });
  });
});
