import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type AdminGenre } from '@bookhaven/api-client';
import { ImportGenresView } from './import-genres-view';

const createMock = vi.fn();

vi.mock('@/entities/genre', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/genre')>()),
  adminGenresApi: {
    create: (...args: unknown[]) => createMock(...args),
  },
}));

vi.mock('@/widgets/wikidata-genre-import', () => ({
  WikidataGenreImportPanel: () => <div>wikidata genre panel</div>,
}));

function makeGenre(overrides: Partial<AdminGenre> = {}): AdminGenre {
  return {
    id: 'g1',
    name: 'повесть',
    slug: 'povest',
    status: 'published',
    wikidata_qid: 'Q49084',
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
      <ImportGenresView />
    </QueryClientProvider>
  );
  return { invalidateSpy };
}

describe('ImportGenresView', () => {
  beforeEach(() => {
    createMock.mockReset();
  });

  it('renders two sections (Вручную, Wikidata) and no file section', () => {
    renderView();

    expect(screen.getByRole('heading', { level: 1, name: 'Импорт: жанры' })).toBeInTheDocument();
    expect(screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)).toEqual([
      'Вручную',
      'Wikidata',
    ]);
    expect(screen.getByRole('region', { name: 'Wikidata' })).toHaveTextContent('wikidata genre panel');
    expect(screen.queryByRole('region', { name: 'Из файла' })).not.toBeInTheDocument();
  });

  it('creates a genre with QID and no status, shows the banner and clears the form', async () => {
    createMock.mockResolvedValue(makeGenre());
    const { invalidateSpy } = renderView();

    expect(screen.queryByLabelText('Статус', { selector: '#status' })).not.toBeInTheDocument();
    const name = screen.getByLabelText('Название *') as HTMLInputElement;
    fireEvent.change(name, { target: { value: 'повесть' } });
    fireEvent.change(screen.getByLabelText('Wikidata QID'), { target: { value: 'Q49084' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() => expect(createMock).toHaveBeenCalled());
    expect(createMock.mock.calls[0][0]).toEqual(
      expect.objectContaining({ name: 'повесть', wikidata_qid: 'Q49084' })
    );
    expect(createMock.mock.calls[0][0]).not.toHaveProperty('status');
    expect(await screen.findByRole('status')).toHaveTextContent('Жанр создан');
    expect(screen.getByRole('link', { name: 'Открыть в каталоге' })).toHaveAttribute(
      'href',
      '/catalog/genres/g1/edit'
    );
    expect(name.value).toBe('');
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'genres'] });
  });

  it('extends a 409 QID conflict with a merged-genre hint and keeps other messages', async () => {
    createMock.mockRejectedValueOnce(new ApiError(409, 'Conflict', 'genre wikidata qid already exists'));
    renderView();

    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'повесть' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
    await waitFor(() =>
      expect(
        screen.getByText('genre wikidata qid already exists (возможно, у слитого жанра)')
      ).toBeInTheDocument()
    );

    createMock.mockRejectedValueOnce(new ApiError(409, 'Conflict', 'slug already exists'));
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
    await waitFor(() => expect(screen.getByText('slug already exists')).toBeInTheDocument());
  });
});
