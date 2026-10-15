import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { LinkScreenWorkDialog } from './link-screen-work-dialog';

const addAdaptationMock = vi.fn();
const addScreenWorkMock = vi.fn();
const listMock = vi.fn();

vi.mock('@/entities/book', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/book')>()),
  adminBooksApi: { addAdaptation: (...a: unknown[]) => addAdaptationMock(...a) },
}));
vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: { addScreenWork: (...a: unknown[]) => addScreenWorkMock(...a) },
}));
vi.mock('@/entities/screen-work', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/screen-work')>()),
  adminScreenWorksApi: { list: (...a: unknown[]) => listMock(...a) },
}));

const work = (id: string, title: string) => ({
  id,
  title,
  original_title: null,
  kind: 'film',
  year: 1964,
  director: null,
  imdb_id: null,
  kinopoisk_id: null,
  qid: 'Q1',
  manually_edited_at: null,
  book_count: 0,
  author_count: 0,
});

function renderDialog(type: 'book' | 'author' = 'book', linkedIds: string[] = []) {
  const onClose = vi.fn();
  const onAdded = vi.fn();
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries');
  render(
    <QueryClientProvider client={queryClient}>
      <LinkScreenWorkDialog target={{ type, id: 'o1', linkedIds }} onClose={onClose} onAdded={onAdded} />
    </QueryClientProvider>
  );
  return { onClose, onAdded, invalidate };
}

describe('LinkScreenWorkDialog', () => {
  beforeEach(() => {
    addAdaptationMock.mockReset();
    addScreenWorkMock.mockReset();
    listMock.mockReset();
  });

  it('posts screen_work_id when a search result is picked', async () => {
    listMock.mockResolvedValue({ items: [work('w1', 'Гамлет'), work('w2', 'Гамлет 2')], total: 2, page: 1, limit: 10 });
    addAdaptationMock.mockResolvedValue({ ...work('w1', 'Гамлет'), work_created: false });
    const { onClose, onAdded, invalidate } = renderDialog('book', ['w2']);

    fireEvent.change(screen.getByLabelText('Поиск фильма'), { target: { value: 'гамлет' } });
    await screen.findByText('Уже привязан');
    expect(listMock).toHaveBeenCalledWith({ search: 'гамлет', limit: 10 });
    fireEvent.click(screen.getByRole('button', { name: 'Выбрать' }));

    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(addAdaptationMock).toHaveBeenCalledWith('o1', { screen_work_id: 'w1' });
    expect(onAdded).toHaveBeenCalledWith('Экранизация «Гамлет» добавлена.');
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['admin', 'books', 'o1', 'adaptations'] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['admin', 'screen-works'] });
  });

  it('creates a new work and blocks invalid input client-side', async () => {
    addAdaptationMock.mockResolvedValue({ ...work('w9', 'Собачье сердце'), work_created: true });
    const { onAdded } = renderDialog('book');

    fireEvent.click(screen.getByRole('button', { name: 'Создать новый' }));
    fireEvent.click(screen.getByRole('button', { name: 'Добавить' }));
    expect(await screen.findByText('Укажите название.')).toBeTruthy();
    expect(addAdaptationMock).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'Собачье сердце' } });
    fireEvent.change(screen.getByLabelText('Год'), { target: { value: '1850' } });
    fireEvent.click(screen.getByRole('button', { name: 'Добавить' }));
    expect(await screen.findByText('Год должен быть от 1880 до 2100.')).toBeTruthy();

    fireEvent.change(screen.getByLabelText('Год'), { target: { value: '1988' } });
    fireEvent.click(screen.getByRole('button', { name: 'Добавить' }));
    await waitFor(() => expect(onAdded).toHaveBeenCalledWith('Экранизация «Собачье сердце» добавлена.'));
    expect(addAdaptationMock.mock.calls[0][1]).toEqual({
      work: expect.objectContaining({ title: 'Собачье сердце', year: 1988, kind: 'film', qid: null }),
    });
  });

  it('adds the QID note when the stored work was linked', async () => {
    addAdaptationMock.mockResolvedValue({ ...work('w1', 'Солярис'), work_created: false });
    const { onAdded } = renderDialog('book');
    fireEvent.click(screen.getByRole('button', { name: 'Создать новый' }));
    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'Солярис' } });
    fireEvent.change(screen.getByLabelText('QID Wikidata'), { target: { value: 'q1' } });
    fireEvent.click(screen.getByRole('button', { name: 'Добавить' }));
    await waitFor(() =>
      expect(onAdded).toHaveBeenCalledWith(
        'Экранизация «Солярис» добавлена. Фильм с этим QID уже был в каталоге — привязан он.'
      )
    );
  });

  it('keeps the dialog open with the duplicate message on 409', async () => {
    addAdaptationMock.mockRejectedValue(new ApiError(409, 'Conflict', 'screen work with this imdb id already exists'));
    const { onClose } = renderDialog('book');
    fireEvent.click(screen.getByRole('button', { name: 'Создать новый' }));
    fireEvent.change(screen.getByLabelText('Название *'), { target: { value: 'Дубль' } });
    fireEvent.change(screen.getByLabelText('IMDb id'), { target: { value: 'tt0069293' } });
    fireEvent.click(screen.getByRole('button', { name: 'Добавить' }));
    expect(
      await screen.findByText('Фильм с таким IMDb id уже есть в каталоге — выберите его в поиске.')
    ).toBeTruthy();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('asks the relation for authors and sends it', async () => {
    listMock.mockResolvedValue({ items: [work('w1', 'Док')], total: 1, page: 1, limit: 10 });
    addScreenWorkMock.mockResolvedValue({ ...work('w1', 'Док'), work_created: false });
    const { onAdded } = renderDialog('author');
    fireEvent.change(screen.getByLabelText('Тип связи'), { target: { value: 'documentary' } });
    fireEvent.change(screen.getByLabelText('Поиск фильма'), { target: { value: 'док' } });
    fireEvent.click(await screen.findByRole('button', { name: 'Выбрать' }));
    await waitFor(() => expect(onAdded).toHaveBeenCalledWith('Фильм «Док» добавлен.'));
    expect(addScreenWorkMock).toHaveBeenCalledWith('o1', { screen_work_id: 'w1', relation: 'documentary' });
  });
});
