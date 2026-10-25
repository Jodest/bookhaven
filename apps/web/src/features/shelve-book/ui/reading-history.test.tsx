import { fireEvent, render as rtlRender, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactElement } from 'react';
import type { BookRead } from '@bookhaven/api-client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { homeKeys } from '@/entities/home';
import { readingChallengeKeys } from '@/entities/reading-challenge';
import { screenWorkKeys } from '@/entities/screen-work';
import { shelfKeys } from '@/entities/shelf';
import { BookShelfActions } from './book-shelf-actions';
import { ReadingHistory } from './reading-history';

let isAuthenticatedValue = true;
let queryClient: QueryClient;

const listMock = vi.fn();
const createMock = vi.fn();
const updateMock = vi.fn();
const removeMock = vi.fn();
const shelvesListMock = vi.fn();

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ isAuthenticated: isAuthenticatedValue, user: { id: 'u1' } }),
}));

vi.mock('@/entities/shelf', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/shelf')>()),
  readsApi: {
    list: (bookId: string) => listMock(bookId),
    create: (input: unknown) => createMock(input),
    update: (id: string, input: unknown) => updateMock(id, input),
    remove: (id: string) => removeMock(id),
  },
  shelvesApi: { list: (p?: unknown) => shelvesListMock(p) },
}));

function render(ui: ReactElement) {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return rtlRender(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

const local = (y: number, m: number, d: number) => new Date(y, m - 1, d, 12).toISOString();

function read(partial: Partial<BookRead>): BookRead {
  return {
    id: 'r',
    bookId: 'b1',
    state: 'finished',
    startedAt: null,
    finishedAt: null,
    abandonedAt: null,
    ...partial,
  };
}

const finished = read({
  id: 'r1',
  startedAt: local(2019, 3, 5),
  finishedAt: local(2019, 4, 20),
});
const abandoned = read({
  id: 'r2',
  state: 'abandoned',
  startedAt: local(2023, 1, 5),
  abandonedAt: local(2023, 2, 1),
});
const open = read({ id: 'r3', state: 'open', startedAt: local(2026, 5, 10) });

async function expand() {
  fireEvent.click(screen.getByRole('button', { name: /История чтения/ }));
  await waitFor(() => expect(listMock).toHaveBeenCalled());
}

beforeEach(() => {
  isAuthenticatedValue = true;
  listMock.mockResolvedValue([finished, abandoned, open]);
  createMock.mockResolvedValue(finished);
  updateMock.mockResolvedValue(finished);
  removeMock.mockResolvedValue({ status: 'ok' });
  shelvesListMock.mockResolvedValue({ items: [], total: 0 });
});

afterEach(() => {
  vi.clearAllMocks();
  vi.restoreAllMocks();
});

describe('ReadingHistory', () => {
  it('a guest sees no history', () => {
    isAuthenticatedValue = false;
    render(<BookShelfActions bookId="b1" slug="b" />);
    expect(screen.queryByText('История чтения')).toBeNull();
  });

  it('a signed-in reader gets the disclosure inside the shelf actions', async () => {
    render(<BookShelfActions bookId="b1" slug="b" />);
    expect(await screen.findByRole('button', { name: /История чтения/ })).toBeTruthy();
    expect(listMock).not.toHaveBeenCalled();
  });

  it('loads the list on expand and shows the lines; the open row has no delete', async () => {
    render(<ReadingHistory bookId="b1" />);
    expect(listMock).not.toHaveBeenCalled();
    await expand();
    expect(await screen.findByText('1-е прочтение: март 2019 — апрель 2019')).toBeTruthy();
    expect(screen.getByText('2-е прочтение: январь 2023 — февраль 2023, брошено')).toBeTruthy();
    expect(screen.getByText('3-е прочтение: с мая 2026 — читаю')).toBeTruthy();
    expect(screen.getAllByRole('button', { name: 'Изменить' })).toHaveLength(3);
    expect(screen.getAllByRole('button', { name: 'Удалить' })).toHaveLength(2);
  });

  it('shows the empty text with the add button', async () => {
    listMock.mockResolvedValue([]);
    render(<ReadingHistory bookId="b1" />);
    await expand();
    expect(await screen.findByText('Вы ещё не отмечали прочтения этой книги.')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Добавить прочтение' })).toBeTruthy();
  });

  it('adds a past reading with finished_on and invalidates the shelf caches', async () => {
    listMock.mockResolvedValue([]);
    render(<ReadingHistory bookId="b1" />);
    const spy = vi.spyOn(queryClient, 'invalidateQueries');
    await expand();
    fireEvent.click(await screen.findByRole('button', { name: 'Добавить прочтение' }));
    fireEvent.change(screen.getByLabelText('Окончание'), { target: { value: '2015-04-20' } });
    fireEvent.click(screen.getByRole('button', { name: 'Добавить' }));
    await waitFor(() =>
      expect(createMock).toHaveBeenCalledWith({
        bookId: 'b1',
        startedOn: null,
        finishedOn: '2015-04-20',
      })
    );
    await waitFor(() => {
      const keys = spy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey));
      expect(keys).toContain(JSON.stringify(shelfKeys.all));
      expect(keys).toContain(JSON.stringify(readingChallengeKeys.all));
      expect(keys).toContain(JSON.stringify(homeKeys.all));
      expect(keys).toContain(JSON.stringify(screenWorkKeys.all));
    });
  });

  it('edit sends only the changed fields', async () => {
    render(<ReadingHistory bookId="b1" />);
    await expand();
    fireEvent.click((await screen.findAllByRole('button', { name: 'Изменить' }))[0]);
    expect((screen.getByLabelText('Начало') as HTMLInputElement).value).toBe('2019-03-05');
    expect((screen.getByLabelText('Окончание') as HTMLInputElement).value).toBe('2019-04-20');
    fireEvent.change(screen.getByLabelText('Окончание'), { target: { value: '2019-05-01' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    await waitFor(() => expect(updateMock).toHaveBeenCalledWith('r1', { finishedOn: '2019-05-01' }));
  });

  it('clearing the start sends null', async () => {
    render(<ReadingHistory bookId="b1" />);
    await expand();
    fireEvent.click((await screen.findAllByRole('button', { name: 'Изменить' }))[0]);
    fireEvent.change(screen.getByLabelText('Начало'), { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    await waitFor(() => expect(updateMock).toHaveBeenCalledWith('r1', { startedOn: null }));
  });

  it('an abandoned row edits «Брошено» as abandonedOn, an open row only the start', async () => {
    render(<ReadingHistory bookId="b1" />);
    await expand();
    const edits = await screen.findAllByRole('button', { name: 'Изменить' });
    fireEvent.click(edits[1]);
    fireEvent.change(screen.getByLabelText('Брошено'), { target: { value: '2023-03-01' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    await waitFor(() => expect(updateMock).toHaveBeenCalledWith('r2', { abandonedOn: '2023-03-01' }));
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Сохранить' })).toBeNull());
    fireEvent.click(screen.getAllByRole('button', { name: 'Изменить' })[2]);
    expect(screen.queryByLabelText('Окончание')).toBeNull();
    expect(screen.queryByLabelText('Брошено')).toBeNull();
    expect(screen.getByLabelText('Начало')).toBeTruthy();
  });

  it('refuses an invalid range and sends nothing', async () => {
    listMock.mockResolvedValue([]);
    render(<ReadingHistory bookId="b1" />);
    await expand();
    fireEvent.click(await screen.findByRole('button', { name: 'Добавить прочтение' }));
    fireEvent.change(screen.getByLabelText('Начало'), { target: { value: '2020-05-10' } });
    fireEvent.change(screen.getByLabelText('Окончание'), { target: { value: '2020-05-01' } });
    fireEvent.click(screen.getByRole('button', { name: 'Добавить' }));
    expect(await screen.findByText('Дата окончания не может быть раньше даты начала')).toBeTruthy();
    expect(createMock).not.toHaveBeenCalled();
  });

  it('a cancelled confirmation sends nothing', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    render(<ReadingHistory bookId="b1" />);
    await expand();
    fireEvent.click((await screen.findAllByRole('button', { name: 'Удалить' }))[0]);
    expect(window.confirm).toHaveBeenCalledWith(
      'Удалить это прочтение из истории? Статус книги не изменится.'
    );
    expect(removeMock).not.toHaveBeenCalled();
  });

  it('a confirmed delete calls remove and invalidates the caches', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(<ReadingHistory bookId="b1" />);
    const spy = vi.spyOn(queryClient, 'invalidateQueries');
    await expand();
    fireEvent.click((await screen.findAllByRole('button', { name: 'Удалить' }))[0]);
    await waitFor(() => expect(removeMock).toHaveBeenCalledWith('r1'));
    await waitFor(() => {
      const keys = spy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey));
      expect(keys).toContain(JSON.stringify(shelfKeys.all));
      expect(keys).toContain(JSON.stringify(readingChallengeKeys.all));
      expect(keys).toContain(JSON.stringify(homeKeys.all));
      expect(keys).toContain(JSON.stringify(screenWorkKeys.all));
    });
  });

  it('shows the save and delete failures', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    removeMock.mockRejectedValue(new Error('boom'));
    updateMock.mockRejectedValue(new Error('boom'));
    render(<ReadingHistory bookId="b1" />);
    await expand();
    fireEvent.click((await screen.findAllByRole('button', { name: 'Удалить' }))[0]);
    expect(await screen.findByText('Не удалось удалить прочтение')).toBeTruthy();
    fireEvent.click(screen.getAllByRole('button', { name: 'Изменить' })[0]);
    fireEvent.change(screen.getByLabelText('Окончание'), { target: { value: '2019-05-01' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    expect(await screen.findByText('Не удалось сохранить прочтение')).toBeTruthy();
    expect(screen.getByText('1-е прочтение: март 2019 — апрель 2019')).toBeTruthy();
  });
});
