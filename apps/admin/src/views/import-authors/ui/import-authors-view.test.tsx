import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminAuthor } from '@bookhaven/api-client';
import { ImportAuthorsView, type ImportAuthorsMethod } from './import-authors-view';

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

vi.mock('@/widgets/ai-author-curation', () => ({
  AiAuthorCurationPanel: () => <div>ai curation panel</div>,
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
    auto_harvest: false,
    last_auto_harvested_at: null,
    ...overrides,
  };
}

function renderView(method: ImportAuthorsMethod = 'manual') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  render(
    <QueryClientProvider client={queryClient}>
      <ImportAuthorsView method={method} />
    </QueryClientProvider>
  );
  return { invalidateSpy };
}

describe('ImportAuthorsView', () => {
  beforeEach(() => {
    createMock.mockReset();
  });

  it('manual renders its h1 and only the create form', () => {
    renderView('manual');

    expect(screen.getByRole('heading', { level: 1, name: 'Импорт авторов: вручную' })).toBeInTheDocument();
    expect(screen.getByText('Создайте автора через форму. Он сразу опубликован.')).toBeInTheDocument();
    expect(screen.getByLabelText('Имя *')).toBeInTheDocument();
    expect(screen.queryByText('wikidata author panel')).not.toBeInTheDocument();
    expect(screen.queryByRole('region')).not.toBeInTheDocument();
  });

  it('wikidata renders its h1 and only the Wikidata panel', () => {
    renderView('wikidata');

    expect(screen.getByRole('heading', { level: 1, name: 'Импорт авторов: Wikidata' })).toBeInTheDocument();
    expect(
      screen.getByText(
        'Найдите автора в Wikidata и импортируйте его (с книгами или без). Импортированные авторы ждут модерации.'
      )
    ).toBeInTheDocument();
    expect(screen.getByText('wikidata author panel')).toBeInTheDocument();
    expect(screen.queryByLabelText('Имя *')).not.toBeInTheDocument();
  });

  it('ai renders «Подбор ИИ» and only the curation panel', () => {
    renderView('ai');

    expect(screen.getByRole('heading', { level: 1, name: 'Подбор ИИ' })).toBeInTheDocument();
    expect(screen.getByText('ai curation panel')).toBeInTheDocument();
    expect(screen.queryByText('wikidata author panel')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Имя *')).not.toBeInTheDocument();
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
      '/catalog/authors/a1/edit'
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
