import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminScreenWorkDetail, ScreenWorkPoster } from '@bookhaven/api-client';
import { ScreenWorkPosterSection } from './screen-work-poster-section';

const uploadMock = vi.fn();
const deleteMock = vi.fn();

vi.mock('@/entities/screen-work', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/screen-work')>()),
  adminScreenWorksApi: {
    uploadPoster: (...a: unknown[]) => uploadMock(...a),
    deletePoster: (...a: unknown[]) => deleteMock(...a),
  },
}));

const makeWork = (patch: Partial<AdminScreenWorkDetail> = {}): AdminScreenWorkDetail => ({
  id: 'w1',
  title: 'Солярис',
  original_title: null,
  kind: 'film',
  year: 1972,
  director: null,
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

const poster = (patch: Partial<ScreenWorkPoster> = {}): ScreenWorkPoster => ({
  url: '/posters/ab/abc.jpg',
  source: 'manual',
  author: null,
  license: null,
  license_url: null,
  source_url: null,
  ...patch,
});

function renderSection(work: AdminScreenWorkDetail) {
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  const invalidate = vi.spyOn(queryClient, 'invalidateQueries');
  render(
    <QueryClientProvider client={queryClient}>
      <ScreenWorkPosterSection work={work} />
    </QueryClientProvider>
  );
  return { invalidate };
}

const pick = (file: File) =>
  fireEvent.change(screen.getByLabelText('Файл постера'), { target: { files: [file] } });

describe('ScreenWorkPosterSection', () => {
  beforeEach(() => {
    uploadMock.mockReset();
    deleteMock.mockReset();
  });
  afterEach(() => vi.restoreAllMocks());

  it('shows "no poster" and the removed-poster hint', () => {
    renderSection(makeWork({ poster_removed: true }));
    expect(screen.getByText('Постера нет.')).toBeTruthy();
    expect(
      screen.getByText('Автоматический постер удалён — синхронизация не подставит его снова.')
    ).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Загрузить постер' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Удалить постер' })).toBeNull();
  });

  it('shows the source of each kind of poster and the Commons attribution', () => {
    const { unmount } = render(
      <QueryClientProvider client={new QueryClient()}>
        <ScreenWorkPosterSection work={makeWork({ poster: poster() })} />
      </QueryClientProvider>
    );
    expect(screen.getByText('Источник: Загружен вручную')).toBeTruthy();
    expect(screen.getByRole('img', { name: 'Солярис' })).toHaveProperty('src', expect.stringContaining('/posters/ab/abc.jpg'));
    expect(screen.getByRole('button', { name: 'Заменить постер' })).toBeTruthy();
    unmount();

    render(
      <QueryClientProvider client={new QueryClient()}>
        <ScreenWorkPosterSection
          work={makeWork({
            poster: poster({
              source: 'commons',
              url: 'https://upload.wikimedia.org/x.jpg',
              author: 'Jane Doe',
              license: 'CC BY-SA 4.0',
              source_url: 'https://commons.wikimedia.org/wiki/File:X.jpg',
            }),
          })}
        />
      </QueryClientProvider>
    );
    expect(screen.getByText('Источник: Wikimedia Commons')).toBeTruthy();
    expect(screen.getByText(/Постер: Jane Doe, CC BY-SA 4\.0,/)).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Wikimedia Commons' }).getAttribute('href')).toBe(
      'https://commons.wikimedia.org/wiki/File:X.jpg'
    );
  });

  it('uploads a valid file and refreshes the work', async () => {
    uploadMock.mockResolvedValue(makeWork({ poster: poster() }));
    const { invalidate } = renderSection(makeWork());
    const file = new File(['x'], 'p.png', { type: 'image/png' });
    pick(file);

    expect(await screen.findByText('Постер загружен.')).toBeTruthy();
    expect(uploadMock).toHaveBeenCalledWith('w1', file);
    await waitFor(() => expect(invalidate).toHaveBeenCalledWith({ queryKey: ['admin', 'screen-works'] }));
  });

  it('rejects a wrong type and a too large file before sending', () => {
    renderSection(makeWork());
    pick(new File(['x'], 'p.gif', { type: 'image/gif' }));
    expect(screen.getByText('Нужен файл JPEG, PNG или WebP.')).toBeTruthy();

    const big = new File(['x'], 'p.jpg', { type: 'image/jpeg' });
    Object.defineProperty(big, 'size', { value: 5 * 1024 * 1024 + 1 });
    pick(big);
    expect(screen.getByText('Файл больше 5 МБ.')).toBeTruthy();
    expect(uploadMock).not.toHaveBeenCalled();
  });

  it('shows a failure message when the upload fails', async () => {
    uploadMock.mockRejectedValue(new Error('boom'));
    renderSection(makeWork());
    pick(new File(['x'], 'p.jpg', { type: 'image/jpeg' }));
    expect(await screen.findByText('Не удалось загрузить постер.')).toBeTruthy();
  });

  it('removes the poster after the confirmation', async () => {
    deleteMock.mockResolvedValue(makeWork({ poster_removed: true }));
    const confirmMock = vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderSection(makeWork({ poster: poster({ source: 'tmdb', url: 'https://image.tmdb.org/t/p/w342/a.jpg' }) }));

    fireEvent.click(screen.getByRole('button', { name: 'Удалить постер' }));

    expect(confirmMock).toHaveBeenCalledWith('Удалить постер фильма «Солярис»?');
    expect(await screen.findByText('Постер удалён.')).toBeTruthy();
    expect(deleteMock).toHaveBeenCalledWith('w1');
  });

  it('does nothing when the removal is not confirmed, and reports a failed removal', async () => {
    const confirmMock = vi.spyOn(window, 'confirm').mockReturnValue(false);
    renderSection(makeWork({ poster: poster() }));
    fireEvent.click(screen.getByRole('button', { name: 'Удалить постер' }));
    expect(deleteMock).not.toHaveBeenCalled();

    confirmMock.mockReturnValue(true);
    deleteMock.mockRejectedValue(new Error('boom'));
    fireEvent.click(screen.getByRole('button', { name: 'Удалить постер' }));
    expect(await screen.findByText('Не удалось удалить постер.')).toBeTruthy();
  });

  it('disables the buttons while a request runs', async () => {
    let resolveUpload: (w: AdminScreenWorkDetail) => void = () => {};
    uploadMock.mockReturnValue(new Promise<AdminScreenWorkDetail>((r) => (resolveUpload = r)));
    renderSection(makeWork({ poster: poster() }));
    pick(new File(['x'], 'p.jpg', { type: 'image/jpeg' }));

    await waitFor(() => {
      expect((screen.getByRole('button', { name: 'Заменить постер' }) as HTMLButtonElement).disabled).toBe(true);
      expect((screen.getByRole('button', { name: 'Удалить постер' }) as HTMLButtonElement).disabled).toBe(true);
    });
    resolveUpload(makeWork({ poster: poster() }));
    await screen.findByText('Постер загружен.');
    expect((screen.getByRole('button', { name: 'Удалить постер' }) as HTMLButtonElement).disabled).toBe(false);
  });
});
