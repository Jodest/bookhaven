import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, AdminHarvestAuthorCandidate } from '@bookhaven/api-client';
import { useWikidataAuthorSearch } from '../model/use-wikidata-author-search';
import { WikidataAuthorSearch } from './wikidata-author-search';

const searchAuthorsMock = vi.fn();

vi.mock('@/entities/harvest', () => ({
  adminHarvestApi: {
    searchAuthors: (...args: unknown[]) => searchAuthorsMock(...args),
  },
}));

function Harness({ emptyHint = 'Ничего не найдено.' }: { emptyHint?: string }) {
  const search = useWikidataAuthorSearch();
  return (
    <>
      <WikidataAuthorSearch
        search={search}
        emptyHint={emptyHint}
        renderActions={(c: AdminHarvestAuthorCandidate) => <button type="button">Действие {c.qid}</button>}
      />
      <button type="button" onClick={search.refresh}>
        refresh
      </button>
    </>
  );
}

function renderHarness(emptyHint?: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <Harness emptyHint={emptyHint} />
    </QueryClientProvider>
  );
}

function searchFor(name: string) {
  fireEvent.change(screen.getByLabelText('Поиск автора по имени'), { target: { value: name } });
  fireEvent.keyDown(screen.getByLabelText('Поиск автора по имени'), { key: 'Enter' });
}

describe('WikidataAuthorSearch', () => {
  beforeEach(() => {
    searchAuthorsMock.mockReset();
  });

  it('renders candidates with QID link, description fallback, existing badge and actions', async () => {
    searchAuthorsMock.mockResolvedValue({
      items: [
        {
          qid: 'Q7200',
          label: 'Лев Толстой',
          description: 'русский писатель',
          existing: { id: 'a1', name: 'Лев Толстой', slug: 'lev-tolstoy', status: 'published' },
        },
        { qid: 'Q171976', label: 'Алексей Толстой', existing: null },
      ],
    });
    renderHarness();
    searchFor('Толстой');

    await waitFor(() => expect(screen.getByText('Лев Толстой')).toBeInTheDocument());
    expect(searchAuthorsMock.mock.calls[0][0]).toBe('Толстой');

    const qidLink = screen.getByRole('link', { name: 'Q7200' });
    expect(qidLink).toHaveAttribute('href', 'https://www.wikidata.org/wiki/Q7200');
    expect(qidLink).toHaveAttribute('target', '_blank');
    expect(qidLink).toHaveAttribute('rel', 'noopener noreferrer');

    expect(screen.getByText('Опубликован')).toBeInTheDocument();
    const open = screen.getByRole('link', { name: 'Открыть' });
    expect(open).toHaveAttribute('href', '/authors/moderation?status=published&search=%D0%9B%D0%B5%D0%B2+%D0%A2%D0%BE%D0%BB%D1%81%D1%82%D0%BE%D0%B9');
    expect(screen.getAllByRole('link', { name: 'Открыть' })).toHaveLength(1);

    expect(screen.getByText('без описания')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Действие Q7200' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Действие Q171976' })).toBeInTheDocument();
  });

  it('shows the emptyHint prop on zero results', async () => {
    searchAuthorsMock.mockResolvedValue({ items: [] });
    renderHarness('Создайте автора вручную.');
    searchFor('Никто');
    await waitFor(() => expect(screen.getByText('Создайте автора вручную.')).toBeInTheDocument());
  });

  it('shows the ApiError message on failure', async () => {
    searchAuthorsMock.mockRejectedValue(new ApiError(502, 'Bad Gateway', 'search source is unavailable'));
    renderHarness();
    searchFor('Толстой');
    await waitFor(() => expect(screen.getByText('search source is unavailable')).toBeInTheDocument());
  });

  it('refresh re-runs the last query', async () => {
    searchAuthorsMock.mockResolvedValue({ items: [] });
    renderHarness();
    fireEvent.click(screen.getByRole('button', { name: 'refresh' }));
    expect(searchAuthorsMock).not.toHaveBeenCalled();

    searchFor('Толстой');
    await waitFor(() => expect(searchAuthorsMock).toHaveBeenCalledTimes(1));
    fireEvent.change(screen.getByLabelText('Поиск автора по имени'), { target: { value: 'другое' } });
    fireEvent.click(screen.getByRole('button', { name: 'refresh' }));
    await waitFor(() => expect(searchAuthorsMock).toHaveBeenCalledTimes(2));
    expect(searchAuthorsMock.mock.calls[1][0]).toBe('Толстой');
  });
});
