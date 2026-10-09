import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, AdminGenreWikidataCandidate } from '@bookhaven/api-client';
import { useWikidataGenreSearch } from '../model/use-wikidata-genre-search';
import { WikidataGenreSearch } from './wikidata-genre-search';

const searchWikidataMock = vi.fn();

vi.mock('@/entities/genre', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre')>()),
  adminGenresApi: {
    searchWikidata: (...args: unknown[]) => searchWikidataMock(...args),
  },
}));

const EMPTY_HINT = 'Литературных жанров не найдено.';
const UNAVAILABLE =
  'Не удалось проверить, что это литературные жанры — проверка будет выполнена при импорте.';

function Harness() {
  const search = useWikidataGenreSearch();
  return (
    <>
      <WikidataGenreSearch
        search={search}
        emptyHint={EMPTY_HINT}
        renderActions={(c: AdminGenreWikidataCandidate) => <button type="button">Действие {c.qid}</button>}
      />
      <button type="button" onClick={search.refresh}>
        refresh
      </button>
    </>
  );
}

function renderHarness() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <Harness />
    </QueryClientProvider>
  );
}

function searchFor(name: string) {
  fireEvent.change(screen.getByLabelText('Название жанра'), { target: { value: name } });
  fireEvent.keyDown(screen.getByLabelText('Название жанра'), { key: 'Enter' });
}

function candidate(overrides: Partial<AdminGenreWikidataCandidate> = {}): AdminGenreWikidataCandidate {
  return {
    qid: 'Q132311',
    label: 'фэнтези',
    description: 'жанр художественной литературы',
    ru_label: 'фэнтези',
    literary_genre: true,
    existing: null,
    ...overrides,
  };
}

describe('WikidataGenreSearch', () => {
  beforeEach(() => {
    searchWikidataMock.mockReset();
  });

  it('renders candidates: ru label, description, QID link, badge, via-alias text and actions', async () => {
    searchWikidataMock.mockResolvedValue({
      class_check: 'ok',
      items: [
        candidate({
          existing: {
            id: 'g1',
            name: 'фэнтези',
            slug: 'fentezi',
            status: 'pending',
            via_alias: { id: 'g2', name: 'фентези', slug: 'fentezi-2' },
          },
        }),
        candidate({ qid: 'Q2', label: 'heroic fantasy', ru_label: null, description: '' }),
      ],
    });
    renderHarness();
    searchFor('  фэнтези ');

    await waitFor(() => expect(screen.getByText('фэнтези')).toBeInTheDocument());
    expect(searchWikidataMock.mock.calls[0][0]).toBe('фэнтези');

    const qidLink = screen.getByRole('link', { name: 'Q132311' });
    expect(qidLink).toHaveAttribute('href', 'https://www.wikidata.org/wiki/Q132311');
    expect(qidLink).toHaveAttribute('target', '_blank');
    expect(qidLink).toHaveAttribute('rel', 'noopener noreferrer');
    expect(screen.getByText('жанр художественной литературы')).toBeInTheDocument();

    expect(screen.getByText('На модерации')).toBeInTheDocument();
    expect(screen.getByText('через слитый жанр «фентези»')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Открыть' })).toHaveAttribute(
      'href',
      `/moderation/genres?${new URLSearchParams({ status: 'pending', search: 'фэнтези' })}`
    );

    // No ru label: the search label plus a hint; empty description fallback.
    expect(screen.getByText('heroic fantasy')).toBeInTheDocument();
    expect(screen.getByText('нет русского названия')).toBeInTheDocument();
    expect(screen.getByText('без описания')).toBeInTheDocument();

    expect(screen.getByRole('button', { name: 'Действие Q132311' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Действие Q2' })).toBeInTheDocument();
    expect(screen.queryByText(UNAVAILABLE)).not.toBeInTheDocument();
  });

  it('warns when the classification was unavailable', async () => {
    searchWikidataMock.mockResolvedValue({
      class_check: 'unavailable',
      items: [candidate({ ru_label: null, literary_genre: null })],
    });
    renderHarness();
    searchFor('фэнтези');
    await waitFor(() => expect(screen.getByText(UNAVAILABLE)).toBeInTheDocument());
  });

  it('shows the empty hint on zero results', async () => {
    searchWikidataMock.mockResolvedValue({ class_check: 'ok', items: [] });
    renderHarness();
    searchFor('ничего');
    await waitFor(() => expect(screen.getByText(EMPTY_HINT)).toBeInTheDocument());
  });

  it('shows the ApiError message on failure', async () => {
    searchWikidataMock.mockRejectedValue(new ApiError(502, 'Bad Gateway', 'wikidata is unavailable; retry later'));
    renderHarness();
    searchFor('фэнтези');
    await waitFor(() => expect(screen.getByText('wikidata is unavailable; retry later')).toBeInTheDocument());
    expect(screen.queryByText(EMPTY_HINT)).not.toBeInTheDocument();
  });

  it('refresh re-runs the last query', async () => {
    searchWikidataMock.mockResolvedValue({ class_check: 'ok', items: [] });
    renderHarness();
    fireEvent.click(screen.getByRole('button', { name: 'refresh' }));
    expect(searchWikidataMock).not.toHaveBeenCalled();

    searchFor('фэнтези');
    await waitFor(() => expect(searchWikidataMock).toHaveBeenCalledTimes(1));
    fireEvent.change(screen.getByLabelText('Название жанра'), { target: { value: 'другое' } });
    fireEvent.click(screen.getByRole('button', { name: 'refresh' }));
    await waitFor(() => expect(searchWikidataMock).toHaveBeenCalledTimes(2));
    expect(searchWikidataMock.mock.calls[1][0]).toBe('фэнтези');
  });
});
