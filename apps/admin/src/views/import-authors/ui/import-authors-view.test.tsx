import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminAuthor } from '@bookhaven/api-client';
import { ImportAuthorsView } from './import-authors-view';

const createMock = vi.fn();

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    create: (...args: unknown[]) => createMock(...args),
  },
}));

vi.mock('@/widgets/wikidata-author-import', () => ({
  WikidataAuthorImportPanel: () => <div>wikidata author panel</div>,
}));

function makeAuthor(overrides: Partial<AdminAuthor> = {}): AdminAuthor {
  return {
    id: 'a1',
    name: 'Новый Автор',
    slug: 'novyi-avtor',
    biography: null,
    photo_url: null,
    birth_date: null,
    death_date: null,
    status: 'published',
    external_source: null,
    external_id: null,
    photo_attribution: null,
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
      <ImportAuthorsView />
    </QueryClientProvider>
  );
  return { invalidateSpy };
}

describe('ImportAuthorsView', () => {
  beforeEach(() => {
    createMock.mockReset();
  });

  it('renders two sections (Вручную, Wikidata) and no file section', () => {
    renderView();

    expect(screen.getByRole('heading', { level: 1, name: 'Импорт: авторы' })).toBeInTheDocument();
    const regions = screen.getAllByRole('region');
    expect(regions).toHaveLength(2);
    expect(regions[0]).toBe(screen.getByRole('region', { name: 'Вручную' }));
    expect(regions[1]).toBe(screen.getByRole('region', { name: 'Wikidata' }));
    expect(screen.getByRole('region', { name: 'Wikidata' })).toHaveTextContent('wikidata author panel');
    expect(screen.queryByRole('region', { name: 'Из файла' })).not.toBeInTheDocument();
  });

  it('creates an author without status, shows the banner with a catalog link and clears the form', async () => {
    createMock.mockResolvedValue(makeAuthor());
    const { invalidateSpy } = renderView();

    expect(screen.queryByLabelText('Статус', { selector: '#status' })).not.toBeInTheDocument();
    const name = screen.getByLabelText('Имя *') as HTMLInputElement;
    fireEvent.change(name, { target: { value: 'Новый Автор' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() => expect(createMock).toHaveBeenCalled());
    expect(createMock.mock.calls[0][0]).toEqual({ name: 'Новый Автор' });
    expect(await screen.findByRole('status')).toHaveTextContent('Автор создан');
    expect(screen.getByRole('link', { name: 'Открыть в каталоге' })).toHaveAttribute(
      'href',
      `/catalog/authors?search=${encodeURIComponent('Новый Автор')}`
    );
    expect(name.value).toBe('');
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['admin', 'authors'] });
  });

  it('shows an API error inline', async () => {
    createMock.mockRejectedValue(new Error('author slug already exists'));
    renderView();

    fireEvent.change(screen.getByLabelText('Имя *'), { target: { value: 'Дубль' } });
    fireEvent.click(screen.getByRole('button', { name: 'Создать' }));

    await waitFor(() => expect(screen.getByText('author slug already exists')).toBeInTheDocument());
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });
});
