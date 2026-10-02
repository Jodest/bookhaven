import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import { MergeAuthorDialog } from './merge-author-dialog';

const listMock = vi.fn();
const mergeMock = vi.fn();

vi.mock('@/entities/author', () => ({
  adminAuthorsApi: {
    list: (...args: unknown[]) => listMock(...args),
    merge: (...args: unknown[]) => mergeMock(...args),
  },
}));

const duplicate = { id: 'dup-1', name: 'Л. Н. Толстой', slug: 'l-n-tolstoy' } as const;
const canonical = { id: 'canon-1', name: 'Лев Толстой', slug: 'tolstoy' } as const;

function renderDialog() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const onClose = vi.fn();
  const onMerged = vi.fn();
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <MergeAuthorDialog duplicate={duplicate as any} onClose={onClose} onMerged={onMerged} />
    </QueryClientProvider>
  );
  return { onClose, onMerged, ...utils };
}

describe('MergeAuthorDialog', () => {
  beforeEach(() => {
    listMock.mockReset();
    mergeMock.mockReset();
    listMock.mockResolvedValue({ items: [canonical], total: 1, page: 1, limit: 20 });
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it('pre-fills the duplicate author read-only and keeps confirm disabled until a distinct canonical author is picked', async () => {
    renderDialog();

    expect(screen.getByText('Л. Н. Толстой')).toBeInTheDocument();

    const confirmButton = screen.getByRole('button', { name: /Слить и удалить/ });
    expect(confirmButton).toBeDisabled();

    fireEvent.change(screen.getByLabelText('Поиск канонического автора'), {
      target: { value: 'Толстой' },
    });

    await waitFor(() => expect(listMock).toHaveBeenCalledWith({ search: 'Толстой', limit: 20 }));

    fireEvent.click(await screen.findByRole('button', { name: 'Лев Толстой' }));

    expect(screen.getByRole('button', { name: /Слить и удалить «Л\. Н\. Толстой»/ })).toBeEnabled();
  });

  it('never offers the duplicate itself as a canonical candidate', async () => {
    listMock.mockResolvedValue({
      items: [duplicate, canonical],
      total: 2,
      page: 1,
      limit: 20,
    });
    renderDialog();

    fireEvent.change(screen.getByLabelText('Поиск канонического автора'), {
      target: { value: 'Толстой' },
    });

    await waitFor(() => expect(listMock).toHaveBeenCalled());
    await screen.findByRole('button', { name: 'Лев Толстой' });

    expect(screen.queryByRole('button', { name: 'Л. Н. Толстой' })).not.toBeInTheDocument();
  });

  it('calls merge with the duplicate and selected canonical IDs, then reports books reassigned on success', async () => {
    mergeMock.mockResolvedValue({ author: canonical, books_reassigned: 5 });
    const { onMerged } = renderDialog();

    fireEvent.change(screen.getByLabelText('Поиск канонического автора'), {
      target: { value: 'Толстой' },
    });
    fireEvent.click(await screen.findByRole('button', { name: 'Лев Толстой' }));
    fireEvent.click(screen.getByRole('button', { name: /Слить и удалить/ }));

    await waitFor(() => expect(mergeMock).toHaveBeenCalledWith('dup-1', 'canon-1'));
    await waitFor(() => expect(onMerged).toHaveBeenCalledWith(5));
  });

  it('shows an inline error on a 400 self-merge response and does not call onMerged', async () => {
    mergeMock.mockRejectedValue(new ApiError(400, 'Bad Request', 'cannot merge an author into itself'));
    const { onMerged } = renderDialog();

    fireEvent.change(screen.getByLabelText('Поиск канонического автора'), {
      target: { value: 'Толстой' },
    });
    fireEvent.click(await screen.findByRole('button', { name: 'Лев Толстой' }));
    fireEvent.click(screen.getByRole('button', { name: /Слить и удалить/ }));

    await waitFor(() =>
      expect(screen.getByText('Нельзя слить автора с самим собой.')).toBeInTheDocument()
    );
    expect(onMerged).not.toHaveBeenCalled();
  });

  it('shows an inline error on a 404 response (author deleted/merged concurrently)', async () => {
    mergeMock.mockRejectedValue(new ApiError(404, 'Not Found', 'author not found'));
    const { onMerged } = renderDialog();

    fireEvent.change(screen.getByLabelText('Поиск канонического автора'), {
      target: { value: 'Толстой' },
    });
    fireEvent.click(await screen.findByRole('button', { name: 'Лев Толстой' }));
    fireEvent.click(screen.getByRole('button', { name: /Слить и удалить/ }));

    await waitFor(() =>
      expect(
        screen.getByText(/Один из авторов не найден/)
      ).toBeInTheDocument()
    );
    expect(onMerged).not.toHaveBeenCalled();
  });
});
