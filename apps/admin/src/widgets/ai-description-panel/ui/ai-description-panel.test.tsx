import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AdminAIDescription, AdminAIDescriptionStatus } from '@bookhaven/api-client';
import { PANEL_POLL_MAX_MS, shouldPoll } from '../lib/poll';
import { AiDescriptionPanel, OWN_DESCRIPTION_NOTICE } from './ai-description-panel';

const getMock = vi.fn();
const statusMock = vi.fn();
const generateMock = vi.fn();

vi.mock('@/entities/ai-description', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/ai-description')>()),
  adminAiDescriptionsApi: {
    get: (...args: unknown[]) => getMock(...args),
    status: (...args: unknown[]) => statusMock(...args),
    generate: (...args: unknown[]) => generateMock(...args),
    approve: vi.fn(),
    discard: vi.fn(),
  },
}));

function status(enabled: boolean): AdminAIDescriptionStatus {
  return { enabled, model: 'claude-sonnet-5-5', prompt_version: 1, missing: 3, drafts: 1, max_bulk: 200 };
}

function record(overrides: Partial<AdminAIDescription> = {}): AdminAIDescription {
  return {
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

function renderPanel(props: { hasOwnDescription?: boolean; bookStatus?: string } = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <AiDescriptionPanel
        bookId="b1"
        hasOwnDescription={props.hasOwnDescription ?? false}
        bookStatus={props.bookStatus ?? 'published'}
      />
    </QueryClientProvider>
  );
}

describe('AiDescriptionPanel', () => {
  beforeEach(() => {
    getMock.mockReset();
    statusMock.mockReset();
    generateMock.mockReset();
    statusMock.mockResolvedValue(status(true));
  });

  it('hidden when the book has its own description and no record', async () => {
    getMock.mockResolvedValue({ ai_description: null });
    const { container } = renderPanel({ hasOwnDescription: true });
    await waitFor(() => expect(getMock).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });

  it('own description with a record: notice, no generation or approval', async () => {
    getMock.mockResolvedValue({ ai_description: record({ status: 'approved', superseded: true }) });
    renderPanel({ hasOwnDescription: true });
    expect(await screen.findByText(OWN_DESCRIPTION_NOTICE)).toBeInTheDocument();
    expect(screen.getByText('Одобрено')).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('no description: shown with «Сгенерировать» when enabled', async () => {
    getMock.mockResolvedValue({ ai_description: null });
    renderPanel();
    expect(await screen.findByRole('button', { name: 'Сгенерировать' })).toBeInTheDocument();
  });

  it('no generation control when the feature is off or the book is rejected', async () => {
    statusMock.mockResolvedValue(status(false));
    getMock.mockResolvedValue({ ai_description: record() });
    renderPanel();
    expect(await screen.findByText('Черновик')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Одобрить' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Сгенерировать/ })).not.toBeInTheDocument();
  });

  it('draft shows text as plain text, stale mark and review actions', async () => {
    getMock.mockResolvedValue({ ai_description: record({ text: '<b>Повесть</b> о враче.', stale: true }) });
    renderPanel();
    expect(await screen.findByText('<b>Повесть</b> о враче.')).toBeInTheDocument();
    expect(screen.getByText('устарел')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Отклонить' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Сгенерировать заново' })).toBeInTheDocument();
  });

  it('regenerating an approved description asks first', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    getMock.mockResolvedValue({ ai_description: record({ status: 'approved' }) });
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'Сгенерировать заново' }));
    expect(confirmSpy).toHaveBeenCalledWith('Одобренное описание будет снято с сайта до проверки нового. Продолжить?');
    expect(generateMock).not.toHaveBeenCalled();
    confirmSpy.mockRestore();
  });

  it('after queueing shows the notice, polls and stops once a new result arrives', async () => {
    getMock.mockResolvedValue({ ai_description: null });
    generateMock.mockResolvedValue({ task_id: 't1' });
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: 'Сгенерировать' }));
    expect(await screen.findByText('Генерация поставлена в очередь.')).toBeInTheDocument();
    expect(generateMock).toHaveBeenCalledWith({ book_ids: ['b1'] });

    // The next 4 s poll returns the new draft; polling and the notice stop.
    getMock.mockResolvedValue({ ai_description: record() });
    expect(await screen.findByText('Черновик', {}, { timeout: 6000 })).toBeInTheDocument();
    expect(screen.queryByText('Генерация поставлена в очередь.')).not.toBeInTheDocument();
    const calls = getMock.mock.calls.length;
    await new Promise((r) => setTimeout(r, 50));
    expect(getMock.mock.calls.length).toBe(calls);
  }, 10000);
});

describe('shouldPoll', () => {
  const start = 1_000_000;
  it('polls until generated_at changes or 3 minutes pass', () => {
    const poll = { previousGeneratedAt: null, startedAt: start };
    expect(shouldPoll(null, null, start)).toBe(false);
    expect(shouldPoll(poll, null, start + 4000)).toBe(true);
    expect(shouldPoll(poll, '2026-10-13T12:00:00Z', start + 4000)).toBe(false);
    expect(shouldPoll(poll, null, start + PANEL_POLL_MAX_MS)).toBe(false);
    expect(shouldPoll({ previousGeneratedAt: 'a', startedAt: start }, 'a', start + 1000)).toBe(true);
  });
});
