import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import type { PastePreview } from '@bookhaven/api-client';
import { pasteBook, pasteLine } from '../lib/fixtures';
import { PasteBooksPanel } from './paste-books-panel';

const previewMock = vi.fn();
const bulkMock = vi.fn();

vi.mock('@/entities/curated-collection', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/curated-collection')>()),
  adminCollectionsApi: {
    previewPaste: (...a: unknown[]) => previewMock(...a),
    bulkAddBooks: (...a: unknown[]) => bulkMock(...a),
  },
}));

function preview(lines: PastePreview['lines'], free = 100): PastePreview {
  const counts = { found: 0, ambiguous: 0, not_found: 0, rejected: 0, in_collection: 0, duplicate: 0, invalid: 0 };
  for (const l of lines) counts[l.status] += 1;
  return { books_count: 1000 - free, free_slots: free, counts, lines };
}

function renderPanel() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <PasteBooksPanel collectionId="c1" />
    </QueryClientProvider>
  );
}

async function runPreview(data: PastePreview, text = 'какой-то текст') {
  previewMock.mockResolvedValue(data);
  renderPanel();
  fireEvent.change(screen.getByLabelText('Список книг'), { target: { value: text } });
  fireEvent.click(screen.getByRole('button', { name: 'Найти в каталоге' }));
  await screen.findByTestId('paste-counts');
}

describe('PasteBooksPanel', () => {
  beforeEach(() => {
    previewMock.mockReset();
    bulkMock.mockReset();
  });

  it('shows the hint and sends the text to the preview', async () => {
    await runPreview(preview([pasteLine(1, 'found', { book: pasteBook('a') })]), 'Мать — Горький');
    expect(previewMock).toHaveBeenCalledWith('c1', 'Мать — Горький');
    expect(
      screen.getByText('Одна книга в строке: Название — Автор. Нумерация и кавычки допускаются. До 500 строк.')
    ).toBeInTheDocument();
  });

  it('ticks found rows by default and counts them', async () => {
    await runPreview(preview([pasteLine(1, 'found', { book: pasteBook('a') })]));
    expect(screen.getByRole('checkbox', { name: 'Добавить строку 1' })).toBeChecked();
    expect(screen.getByRole('button', { name: 'Добавить выбранные (1)' })).toBeEnabled();
  });

  it('ticks an ambiguous row once a candidate is picked', async () => {
    await runPreview(preview([pasteLine(1, 'ambiguous', { candidates: [pasteBook('g'), pasteBook('c')] })]));
    const box = screen.getByRole('checkbox', { name: 'Добавить строку 1' });
    expect(box).not.toBeChecked();
    expect(box).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Добавить выбранные (0)' })).toBeDisabled();

    fireEvent.click(screen.getByRole('radio', { name: /Книга g/ }));
    expect(screen.getByRole('checkbox', { name: 'Добавить строку 1' })).toBeChecked();
    expect(screen.getByRole('button', { name: 'Добавить выбранные (1)' })).toBeEnabled();
  });

  it('does not let rejected or held candidates be picked', async () => {
    await runPreview(
      preview([
        pasteLine(1, 'ambiguous', {
          candidates: [
            pasteBook('ok'),
            pasteBook('rej', { status: 'rejected' }),
            pasteBook('held', { in_collection: true }),
          ],
        }),
      ])
    );
    expect(screen.getByRole('radio', { name: /Книга rej/ })).toBeDisabled();
    expect(screen.getByRole('radio', { name: /Книга held/ })).toBeDisabled();
    expect(screen.getByRole('radio', { name: /Книга ok/ })).toBeEnabled();
  });

  it('gives a not_found row no checkbox, a message and a catalog link', async () => {
    await runPreview(preview([pasteLine(1, 'not_found', { text: 'Нет такой' })]));
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    expect(screen.getByText('Не найдено — сначала добавьте книгу в каталог')).toBeInTheDocument();
    const link = screen.getByRole('link', { name: 'Добавить в каталог' });
    expect(link).toHaveAttribute('href', '/import/books/manual');
    expect(link).toHaveAttribute('target', '_blank');
  });

  it('disables apply when more rows are ticked than free places', async () => {
    const lines = [1, 2, 3].map((n) => pasteLine(n, 'found', { book: pasteBook(`b${n}`) }));
    await runPreview(preview(lines, 2));
    expect(screen.getByRole('button', { name: 'Добавить выбранные (3)' })).toBeDisabled();
    expect(screen.getByText('Свободно мест: 2 — снимите лишние отметки')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Добавить строку 3' }));
    expect(screen.getByRole('button', { name: 'Добавить выбранные (2)' })).toBeEnabled();
  });

  it('unticks the later row of two pointing at the same book', async () => {
    const lines = [pasteLine(1, 'found', { book: pasteBook('a') }), pasteLine(5, 'found', { book: pasteBook('a') })];
    await runPreview(preview(lines));
    expect(screen.getByRole('checkbox', { name: 'Добавить строку 5' })).not.toBeChecked();
    expect(screen.getByText('Повтор строки 1')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Добавить выбранные (1)' })).toBeInTheDocument();
  });

  it('hides the preview when the text is edited', async () => {
    await runPreview(preview([pasteLine(1, 'found', { book: pasteBook('a') })]));
    fireEvent.change(screen.getByLabelText('Список книг'), { target: { value: 'другое' } });
    expect(screen.queryByTestId('paste-counts')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Добавить выбранные/ })).not.toBeInTheDocument();
  });

  it('adds the chosen books in order, at a position, and reports the result', async () => {
    const lines = [
      pasteLine(1, 'found', { text: 'Первая', book: pasteBook('a') }),
      pasteLine(2, 'found', { text: 'Вторая', book: pasteBook('b') }),
      pasteLine(3, 'not_found', { text: 'Нет такой' }),
    ];
    bulkMock.mockResolvedValue({
      added: [{ book_id: 'a', position: 3 }],
      skipped: [{ book_id: 'b', reason: 'already_in_collection' }],
      books_count: 3,
    });
    await runPreview(preview(lines));
    fireEvent.click(screen.getByRole('radio', { name: 'На позицию' }));
    fireEvent.change(screen.getByLabelText('Позиция'), { target: { value: '3' } });
    fireEvent.click(screen.getByRole('button', { name: 'Добавить выбранные (2)' }));

    await waitFor(() => expect(bulkMock).toHaveBeenCalledWith('c1', ['a', 'b'], 3));
    const report = await screen.findByTestId('paste-report');
    expect(within(report).getByText('Добавлено: 1')).toBeInTheDocument();
    expect(within(report).getByText('Пропущено: 1')).toBeInTheDocument();
    expect(within(report).getByText('Вторая — уже в подборке')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Добавить выбранные/ })).not.toBeInTheDocument();
  });

  it('appends at the end without a position', async () => {
    bulkMock.mockResolvedValue({ added: [{ book_id: 'a', position: 1 }], skipped: [], books_count: 1 });
    await runPreview(preview([pasteLine(1, 'found', { book: pasteBook('a') })]));
    fireEvent.click(screen.getByRole('button', { name: 'Добавить выбранные (1)' }));
    await waitFor(() => expect(bulkMock).toHaveBeenCalledWith('c1', ['a'], undefined));
  });

  it('copies the not_found lines', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    await runPreview(preview([pasteLine(1, 'not_found', { text: 'А' }), pasteLine(2, 'not_found', { text: 'Б' })]));
    fireEvent.click(screen.getByRole('button', { name: 'Скопировать ненайденные' }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('А\nБ'));
  });

  it('shows a readable message when the collection is full', async () => {
    bulkMock.mockRejectedValue(new ApiError(422, 'Unprocessable', 'collection is full'));
    await runPreview(preview([pasteLine(1, 'found', { book: pasteBook('a') })]));
    fireEvent.click(screen.getByRole('button', { name: 'Добавить выбранные (1)' }));
    expect(await screen.findByText('Не хватает мест в подборке (максимум 1000 книг)')).toBeInTheDocument();
  });

  it('shows the too-many-lines message from the preview', async () => {
    previewMock.mockRejectedValue(new ApiError(422, 'Unprocessable', 'too many lines (max 500)'));
    renderPanel();
    fireEvent.change(screen.getByLabelText('Список книг'), { target: { value: 'x' } });
    fireEvent.click(screen.getByRole('button', { name: 'Найти в каталоге' }));
    expect(await screen.findByText('Слишком много строк — не больше 500')).toBeInTheDocument();
  });
});
