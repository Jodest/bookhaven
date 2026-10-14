import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminAIDescriptionItem, AdminAIDescriptionStatus } from '@bookhaven/api-client';
import { AiDescriptionModerationView } from './ai-description-moderation-view';

const pushMock = vi.fn();
let searchParamsValue = new URLSearchParams();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => '/moderation/descriptions',
  useSearchParams: () => searchParamsValue,
}));

const listMock = vi.fn();
const statusMock = vi.fn();
const approveMock = vi.fn();
const discardMock = vi.fn();
const generateMock = vi.fn();

vi.mock('@/entities/ai-description', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/ai-description')>()),
  adminAiDescriptionsApi: {
    list: (...args: unknown[]) => listMock(...args),
    status: (...args: unknown[]) => statusMock(...args),
    approve: (...args: unknown[]) => approveMock(...args),
    discard: (...args: unknown[]) => discardMock(...args),
    generate: (...args: unknown[]) => generateMock(...args),
  },
}));

function status(enabled: boolean): AdminAIDescriptionStatus {
  return { enabled, model: 'claude-sonnet-5-5', prompt_version: 1, missing: 134, drafts: 6, max_bulk: 200 };
}

function item(overrides: Partial<AdminAIDescriptionItem> = {}): AdminAIDescriptionItem {
  return {
    book: { id: 'b1', title: 'Записки юного врача', authors: ['Михаил Булгаков'], published_year: 1925, status: 'published' },
    status: 'draft',
    text: 'Повесть рассказывает о молодом враче.',
    error_code: null,
    model: 'claude-sonnet-5-5',
    prompt_version: 1,
    generated_at: '2026-10-13T12:00:00Z',
    reviewed_at: null,
    edited: false,
    stale: false,
    superseded: false,
    ...overrides,
  };
}

function renderView() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <AiDescriptionModerationView />
    </QueryClientProvider>
  );
}

describe('AiDescriptionModerationView', () => {
  beforeEach(() => {
    pushMock.mockReset();
    listMock.mockReset();
    statusMock.mockReset();
    approveMock.mockReset();
    discardMock.mockReset();
    generateMock.mockReset();
    searchParamsValue = new URLSearchParams();
    statusMock.mockResolvedValue(status(true));
    listMock.mockResolvedValue({ items: [item()], total: 1, page: 1, limit: 20 });
  });

  it('defaults to drafts and lists rows with plain text, link and marks', async () => {
    listMock.mockResolvedValue({ items: [item({ stale: true, text: '<i>Повесть</i>' })], total: 1, page: 1, limit: 20 });
    renderView();
    expect(await screen.findByText('<i>Повесть</i>')).toBeInTheDocument();
    expect(listMock).toHaveBeenCalledWith({ status: 'draft', page: 1, limit: 20 });
    expect(screen.getByRole('link', { name: 'Записки юного врача' })).toHaveAttribute(
      'href',
      '/catalog/books/b1/edit?back=%2Fmoderation%2Fdescriptions'
    );
    expect(screen.getByText('Михаил Булгаков · 1925')).toBeInTheDocument();
    expect(screen.getByText('устарел')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Черновики' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('reads the status filter from the URL and pushes changes', async () => {
    searchParamsValue = new URLSearchParams('status=approved&page=2');
    listMock.mockResolvedValue({ items: [item({ status: 'approved' })], total: 21, page: 2, limit: 20 });
    renderView();
    await waitFor(() => expect(listMock).toHaveBeenCalledWith({ status: 'approved', page: 2, limit: 20 }));
    fireEvent.click(screen.getByRole('button', { name: 'Ошибки' }));
    expect(pushMock).toHaveBeenCalledWith('/moderation/descriptions?status=failed');
    fireEvent.click(screen.getByRole('button', { name: 'Черновики' }));
    expect(pushMock).toHaveBeenLastCalledWith('/moderation/descriptions?');
  });

  it('an unknown status falls back to drafts', async () => {
    searchParamsValue = new URLSearchParams('status=bogus');
    renderView();
    await waitFor(() => expect(listMock).toHaveBeenCalledWith({ status: 'draft', page: 1, limit: 20 }));
  });

  it('row approve and discard, one row at a time', async () => {
    approveMock.mockResolvedValue(item({ status: 'approved' }));
    discardMock.mockResolvedValue(item({ status: 'discarded' }));
    listMock.mockResolvedValue({
      items: [item(), item({ book: { ...item().book, id: 'b2', title: 'Другая' } })],
      total: 2,
      page: 1,
      limit: 20,
    });
    renderView();
    const approveButtons = await screen.findAllByRole('button', { name: 'Одобрить' });
    expect(approveButtons).toHaveLength(2);
    fireEvent.click(approveButtons[0]);
    await waitFor(() => expect(approveMock).toHaveBeenCalledWith('b1', undefined));
    expect(await screen.findByText('Описание ИИ одобрено и показывается на сайте с пометкой.')).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole('button', { name: 'Отклонить' })[1]);
    await waitFor(() => expect(discardMock).toHaveBeenCalledWith('b2'));
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('a superseded row offers no actions', async () => {
    listMock.mockResolvedValue({ items: [item({ superseded: true })], total: 1, page: 1, limit: 20 });
    renderView();
    expect(await screen.findByText('есть своё описание')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Одобрить' })).not.toBeInTheDocument();
  });

  it('toolbar: count and confirmed bulk generation when enabled', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    generateMock.mockResolvedValue({ task_id: 't1' });
    renderView();
    expect(await screen.findByText('Книг без описания: 134')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Сгенерировать для книг без описания' }));
    expect(confirmSpy).toHaveBeenCalledWith('Сгенерировать описания ИИ для книг без описания (до 200 из 134)?');
    expect(await screen.findByText('Генерация поставлена в очередь.')).toBeInTheDocument();
    confirmSpy.mockRestore();
  });

  it('toolbar hidden when the feature is off', async () => {
    statusMock.mockResolvedValue(status(false));
    renderView();
    await screen.findByText('Повесть рассказывает о молодом враче.');
    await waitFor(() => expect(statusMock).toHaveBeenCalled());
    expect(screen.queryByText(/Книг без описания/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Сгенерировать для книг без описания' })).not.toBeInTheDocument();
    // Review still works while off.
    expect(screen.getByRole('button', { name: 'Одобрить' })).toBeInTheDocument();
  });
});
