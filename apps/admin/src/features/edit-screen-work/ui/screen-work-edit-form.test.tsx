import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, type AdminScreenWorkDetail } from '@bookhaven/api-client';
import { ScreenWorkEditForm } from './screen-work-edit-form';

const updateMock = vi.fn();

vi.mock('@/entities/screen-work', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/screen-work')>()),
  adminScreenWorksApi: { update: (...a: unknown[]) => updateMock(...a) },
}));

const makeWork = (patch: Partial<AdminScreenWorkDetail> = {}): AdminScreenWorkDetail => ({
  id: 'w1',
  title: 'Солярис',
  original_title: null,
  kind: 'film',
  year: 1972,
  director: 'Тарковский',
  imdb_id: null,
  kinopoisk_id: null,
  qid: null,
  manually_edited_at: null,
  book_count: 0,
  author_count: 0,
  books: [],
  authors: [],
  poster: null,
  poster_removed: false,
  ...patch,
});

function renderForm(work = makeWork()) {
  const onSaved = vi.fn();
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <ScreenWorkEditForm work={work} onSaved={onSaved} onCancel={vi.fn()} />
    </QueryClientProvider>
  );
  return { onSaved };
}

describe('ScreenWorkEditForm', () => {
  beforeEach(() => {
    updateMock.mockReset();
  });

  it('reduces a pasted Kinopoisk link to the id on save', async () => {
    const updated = makeWork({ kinopoisk_id: '43911' });
    updateMock.mockResolvedValue(updated);
    const { onSaved } = renderForm();
    fireEvent.change(screen.getByLabelText('Кинопоиск id'), {
      target: { value: 'https://www.kinopoisk.ru/film/43911/' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(updated));
    expect(updateMock).toHaveBeenCalledWith('w1', expect.objectContaining({ kinopoisk_id: '43911', year: 1972 }));
  });

  it('validates before sending', async () => {
    renderForm();
    fireEvent.change(screen.getByLabelText('Год'), { target: { value: '1850' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    expect(await screen.findByText('Год должен быть от 1880 до 2100.')).toBeTruthy();
    expect(updateMock).not.toHaveBeenCalled();
  });

  it('makes a stored QID read-only', () => {
    renderForm(makeWork({ qid: 'Q181069' }));
    expect((screen.getByLabelText('QID Wikidata') as HTMLInputElement).readOnly).toBe(true);
  });

  it('shows the taken QID error', async () => {
    updateMock.mockRejectedValue(new ApiError(409, 'Conflict', 'screen work with this qid already exists'));
    renderForm();
    fireEvent.change(screen.getByLabelText('QID Wikidata'), { target: { value: 'Q5' } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    expect(await screen.findByText('Фильм с таким QID уже есть в каталоге.')).toBeTruthy();
  });

  it('shows the generic failure', async () => {
    updateMock.mockRejectedValue(new ApiError(500, 'Server Error', 'boom'));
    renderForm();
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    expect(await screen.findByText('Не удалось сохранить фильм.')).toBeTruthy();
  });
});
