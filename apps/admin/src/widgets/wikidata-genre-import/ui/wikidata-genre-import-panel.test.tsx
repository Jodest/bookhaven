import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import type {
  AdminGenre,
  AdminGenreImportResult,
  AdminGenreWikidataCandidate,
  AdminGenreWikidataSearchResult,
} from '@bookhaven/api-client';
import { WikidataGenreImportPanel } from './wikidata-genre-import-panel';

const importMock = vi.fn();
const searchMock = vi.fn();

vi.mock('@/entities/genre', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre')>()),
  adminGenresApi: {
    searchWikidata: (...args: unknown[]) => searchMock(...args),
    importWikidata: (...args: unknown[]) => importMock(...args),
  },
}));

function genre(overrides: Partial<AdminGenre> = {}): AdminGenre {
  return { id: 'g1', name: 'фэнтези', slug: 'fentezi', status: 'pending', wikidata_qid: 'Q132311', ...overrides };
}

function importResult(overrides: Partial<AdminGenreImportResult> = {}): AdminGenreImportResult {
  return {
    outcome: 'created',
    match: null,
    qid_backfilled: false,
    qid_mismatch: null,
    via_alias: null,
    genre: genre(),
    ...overrides,
  };
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

function renderView() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  render(
    <QueryClientProvider client={queryClient}>
      <WikidataGenreImportPanel />
    </QueryClientProvider>
  );
  return { invalidateSpy };
}

async function searchWith(result: AdminGenreWikidataSearchResult, firstText: string) {
  searchMock.mockResolvedValue(result);
  fireEvent.change(screen.getByLabelText('Название жанра в Wikidata'), { target: { value: 'фэнтези' } });
  fireEvent.click(screen.getByRole('button', { name: 'Найти' }));
  await waitFor(() => expect(screen.getByText(firstText)).toBeInTheDocument());
}

describe('WikidataGenreImportPanel', () => {
  beforeEach(() => {
    importMock.mockReset();
    searchMock.mockReset();
  });

  it('renders the search without a page heading of its own', () => {
    renderView();
    expect(screen.queryByRole('heading', { level: 1 })).not.toBeInTheDocument();
    expect(screen.getByLabelText('Название жанра в Wikidata')).toBeInTheDocument();
  });

  it('the empty-result hint links «Вручную» to the manual sub-page', async () => {
    renderView();
    searchMock.mockResolvedValue({ class_check: 'ok', items: [] });
    fireEvent.change(screen.getByLabelText('Название жанра в Wikidata'), { target: { value: 'нет такого' } });
    fireEvent.click(screen.getByRole('button', { name: 'Найти' }));

    const link = await screen.findByRole('link', { name: '«Вручную»' });
    expect(link).toHaveAttribute('href', '/import/genres/manual');
    expect(link.parentElement).toHaveTextContent(
      'Литературных жанров не найдено. Проверьте написание или создайте жанр вручную в разделе «Вручную».'
    );
  });

  it('imports a candidate, shows the banner with links, invalidates caches and refreshes the search', async () => {
    importMock.mockResolvedValue(importResult());
    const { invalidateSpy } = renderView();
    await searchWith({ class_check: 'ok', items: [candidate()] }, 'жанр художественной литературы');

    fireEvent.click(screen.getByRole('button', { name: 'Импортировать' }));

    await waitFor(() =>
      expect(screen.getByText('Жанр «фэнтези» импортирован и ждёт модерации.')).toBeInTheDocument()
    );
    expect(importMock).toHaveBeenCalledWith('Q132311');
    expect(screen.getByRole('link', { name: 'Открыть' })).toHaveAttribute(
      'href',
      `/moderation/genres?${new URLSearchParams({ status: 'pending', search: 'фэнтези' })}`
    );
    expect(screen.getByRole('link', { name: 'К модерации жанров' })).toHaveAttribute('href', '/moderation/genres');
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'genres'] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'dashboard'] });
    await waitFor(() => expect(searchMock).toHaveBeenCalledTimes(2));
    expect(searchMock.mock.calls[1][0]).toBe('фэнтези');
  });

  it('shows a destructive banner for a rejected outcome', async () => {
    importMock.mockResolvedValue(
      importResult({ outcome: 'rejected', match: 'slug', qid_backfilled: true, genre: genre({ status: 'rejected' }) })
    );
    renderView();
    await searchWith({ class_check: 'ok', items: [candidate()] }, 'жанр художественной литературы');

    fireEvent.click(screen.getByRole('button', { name: 'Импортировать' }));

    const banner = await screen.findByRole('status');
    expect(banner).toHaveTextContent(
      'Жанр «фэнтези» был отклонён ранее и повторно не создаётся. QID привязан к нему.'
    );
    expect(banner.className).toContain('text-destructive');
  });

  it('shows the not_a_literary_genre message on 422', async () => {
    importMock.mockRejectedValue(new ApiError(422, 'Unprocessable Entity', 'x', '', 'not_a_literary_genre'));
    renderView();
    await searchWith({ class_check: 'ok', items: [candidate()] }, 'жанр художественной литературы');

    fireEvent.click(screen.getByRole('button', { name: 'Импортировать' }));

    await waitFor(() =>
      expect(screen.getByText('Элемент Wikidata не является литературным жанром — импорт отменён.')).toBeInTheDocument()
    );
    expect(searchMock).toHaveBeenCalledTimes(1);
  });

  it('actions per candidate state: new, existing, rejected, no ru label', async () => {
    renderView();
    await searchWith(
      {
        class_check: 'ok',
        items: [
          candidate({ qid: 'Q1', description: 'новый' }),
          candidate({
            qid: 'Q2',
            ru_label: 'роман',
            description: 'есть',
            existing: { id: 'g2', name: 'роман', slug: 'roman', status: 'published', via_alias: null },
          }),
          candidate({
            qid: 'Q3',
            ru_label: 'мусор',
            description: 'отклонён',
            existing: { id: 'g3', name: 'мусор', slug: 'musor', status: 'rejected', via_alias: null },
          }),
          candidate({ qid: 'Q4', label: 'heroic fantasy', ru_label: null, description: 'без ru' }),
        ],
      },
      'новый'
    );

    expect(screen.getAllByRole('button', { name: 'Импортировать' })).toHaveLength(1);
    expect(screen.getByText('Уже в каталоге')).toBeInTheDocument();
    expect(screen.getByText('Отклонён ранее')).toBeInTheDocument();
    expect(screen.getByText('Нет русского названия')).toBeInTheDocument();
  });

  it('unchecked candidates (class_check unavailable) can be imported', async () => {
    importMock.mockResolvedValue(importResult());
    renderView();
    await searchWith(
      { class_check: 'unavailable', items: [candidate({ ru_label: null, literary_genre: null })] },
      'жанр художественной литературы'
    );

    expect(screen.queryByText('Нет русского названия')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Импортировать' }));
    await waitFor(() => expect(importMock).toHaveBeenCalledWith('Q132311'));
  });

  it('disables the clicked button while the import is pending', async () => {
    let resolve: (r: AdminGenreImportResult) => void = () => {};
    importMock.mockReturnValue(new Promise<AdminGenreImportResult>((r) => (resolve = r)));
    renderView();
    await searchWith(
      { class_check: 'ok', items: [candidate(), candidate({ qid: 'Q2', description: 'второй' })] },
      'второй'
    );

    const [first, second] = screen.getAllByRole('button', { name: 'Импортировать' });
    fireEvent.click(first);
    await waitFor(() => expect(first).toBeDisabled());
    expect(second).not.toBeDisabled();

    resolve(importResult());
    await waitFor(() => expect(screen.getByRole('status')).toBeInTheDocument());
  });
});
