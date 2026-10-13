import { act, renderHook } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import type { AdminAuthor, AdminCurationOption } from '@bookhaven/api-client';
import { useTakeCuratedAuthors } from './use-take-curated-authors';

const importMock = vi.fn();
const getMock = vi.fn();
const updateMock = vi.fn();
const triggerMock = vi.fn();

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    importWikidata: (...args: unknown[]) => importMock(...args),
    get: (...args: unknown[]) => getMock(...args),
    update: (...args: unknown[]) => updateMock(...args),
  },
}));

vi.mock('@/entities/harvest', () => ({
  HARVEST_JOBS_QUERY_KEY: ['admin', 'harvest', 'jobs'],
  adminHarvestApi: {
    trigger: (...args: unknown[]) => triggerMock(...args),
  },
}));

function author(qid: string, overrides: Partial<AdminAuthor> = {}): AdminAuthor {
  return {
    id: `id-${qid}`,
    name: `Автор ${qid}`,
    slug: `a-${qid}`,
    biography: null,
    photo_url: null,
    birth_date: null,
    death_date: null,
    status: 'pending',
    external_source: 'wikidata',
    external_id: qid,
    photo_attribution: null,
    auto_harvest: false,
    last_auto_harvested_at: null,
    ...overrides,
  };
}

function option(qid: string): AdminCurationOption {
  return { qid, label: `Автор ${qid}`, description: '', existing: null };
}

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  const hook = renderHook(() => useTakeCuratedAuthors(['admin', 'ai-curator', 'requests', 'r1']), { wrapper });
  return { hook, invalidateSpy };
}

describe('useTakeCuratedAuthors', () => {
  beforeEach(() => {
    importMock.mockReset();
    getMock.mockReset();
    updateMock.mockReset();
    triggerMock.mockReset();
  });

  it('imports sequentially, harvests once and flags created/existing authors', async () => {
    importMock.mockImplementation(async (qid: string) => {
      if (qid === 'Q2') return { outcome: 'existing', match: 'qid', author: author('Q2', { status: 'published' }) };
      return { outcome: 'created', match: null, author: author(qid) };
    });
    getMock.mockImplementation(async (id: string) => author(id.replace('id-', '')));
    updateMock.mockResolvedValue({});
    triggerMock.mockResolvedValue({ task_id: 't1' });

    const { hook, invalidateSpy } = setup();
    let summary: Awaited<ReturnType<typeof hook.result.current.run>> | undefined;
    await act(async () => {
      summary = await hook.result.current.run({
        options: [option('Q1'), option('Q2'), option('Q3')],
        withBooks: true,
        autoHarvest: true,
      });
    });

    expect(importMock.mock.calls.map((c) => c[0])).toEqual(['Q1', 'Q2', 'Q3']);
    expect(triggerMock).toHaveBeenCalledTimes(1);
    expect(triggerMock).toHaveBeenCalledWith({ author_qids: ['Q1', 'Q2', 'Q3'], limit: 900 });
    expect(updateMock).toHaveBeenCalledTimes(3);
    const [id, body] = updateMock.mock.calls[1];
    expect(id).toBe('id-Q2');
    expect(body).toMatchObject({ name: 'Автор Q2', auto_harvest: true });
    expect(summary!.harvest).toEqual({ state: 'started', count: 3 });
    expect(summary!.results.map((r) => r.autoHarvest.state)).toEqual(['set', 'set', 'set']);
    expect(hook.result.current.progress).toEqual({ done: 3, total: 3 });
    const keys = invalidateSpy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey));
    expect(keys).toEqual(
      expect.arrayContaining([
        JSON.stringify(['admin', 'authors']),
        JSON.stringify(['admin', 'dashboard']),
        JSON.stringify(['admin', 'harvest', 'jobs']),
        JSON.stringify(['admin', 'ai-curator', 'requests', 'r1']),
      ])
    );
  });

  it('a rejected outcome is not harvested or flagged; an import error continues the batch', async () => {
    importMock.mockImplementation(async (qid: string) => {
      if (qid === 'Q1') return { outcome: 'rejected', match: 'qid', author: author('Q1', { status: 'rejected' }) };
      if (qid === 'Q2') throw new ApiError(502, 'Bad Gateway', 'wikidata down');
      return { outcome: 'created', match: null, author: author(qid) };
    });
    getMock.mockImplementation(async () => author('Q3'));
    updateMock.mockResolvedValue({});
    triggerMock.mockResolvedValue({ task_id: 't1' });

    const { hook } = setup();
    let summary: Awaited<ReturnType<typeof hook.result.current.run>> | undefined;
    await act(async () => {
      summary = await hook.result.current.run({
        options: [option('Q1'), option('Q2'), option('Q3')],
        withBooks: true,
        autoHarvest: true,
      });
    });

    expect(importMock).toHaveBeenCalledTimes(3);
    expect(triggerMock).toHaveBeenCalledWith({ author_qids: ['Q3'], limit: 300 });
    expect(updateMock).toHaveBeenCalledTimes(1);
    expect(updateMock.mock.calls[0][0]).toBe('id-Q3');
    const [rejected, failed, created] = summary!.results;
    expect(rejected.outcome).toBe('rejected');
    expect(rejected.autoHarvest.state).toBe('not_requested');
    expect(failed.error).toBe('wikidata down');
    expect(created.autoHarvest.state).toBe('set');
  });

  it('skips the harvest when no QIDs are left and does not re-flag', async () => {
    importMock.mockResolvedValue({ outcome: 'rejected', match: 'qid', author: author('Q1', { status: 'rejected' }) });
    const { hook } = setup();
    let summary: Awaited<ReturnType<typeof hook.result.current.run>> | undefined;
    await act(async () => {
      summary = await hook.result.current.run({ options: [option('Q1')], withBooks: true, autoHarvest: true });
    });
    expect(triggerMock).not.toHaveBeenCalled();
    expect(updateMock).not.toHaveBeenCalled();
    expect(summary!.harvest).toEqual({ state: 'skipped' });
  });

  it('an already flagged author is not patched; without books there is no harvest', async () => {
    importMock.mockResolvedValue({ outcome: 'existing', match: 'qid', author: author('Q1', { auto_harvest: true }) });
    const { hook } = setup();
    let summary: Awaited<ReturnType<typeof hook.result.current.run>> | undefined;
    await act(async () => {
      summary = await hook.result.current.run({ options: [option('Q1')], withBooks: false, autoHarvest: true });
    });
    expect(triggerMock).not.toHaveBeenCalled();
    expect(getMock).not.toHaveBeenCalled();
    expect(updateMock).not.toHaveBeenCalled();
    expect(summary!.harvest).toEqual({ state: 'not_requested' });
    expect(summary!.results[0].autoHarvest).toEqual({ state: 'already' });
  });
});
