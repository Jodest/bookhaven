import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type {
  AdminAuthor,
  AdminCurationCandidate,
  AdminCurationRequest,
  AdminCurationStatus,
} from '@bookhaven/api-client';
import { AiAuthorCurationPanel } from './ai-author-curation-panel';

let searchParamsValue = new URLSearchParams();
const replaceMock = vi.fn();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: replaceMock }),
  usePathname: () => '/import/authors/ai',
  useSearchParams: () => searchParamsValue,
}));

const statusMock = vi.fn();
const listMock = vi.fn();
const getMock = vi.fn();
const createMock = vi.fn();
const importMock = vi.fn();
const triggerMock = vi.fn();

vi.mock('@/entities/ai-curation', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/ai-curation')>()),
  adminAiCuratorApi: {
    status: (...a: unknown[]) => statusMock(...a),
    list: (...a: unknown[]) => listMock(...a),
    get: (...a: unknown[]) => getMock(...a),
    create: (...a: unknown[]) => createMock(...a),
  },
}));

vi.mock('@/entities/author', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/author')>()),
  adminAuthorsApi: {
    importWikidata: (...a: unknown[]) => importMock(...a),
    get: vi.fn(),
    update: vi.fn(),
  },
}));

vi.mock('@/entities/harvest', () => ({
  HARVEST_JOBS_QUERY_KEY: ['admin', 'harvest', 'jobs'],
  adminHarvestApi: { trigger: (...a: unknown[]) => triggerMock(...a) },
}));

function status(overrides: Partial<AdminCurationStatus> = {}): AdminCurationStatus {
  return {
    enabled: true,
    model: 'claude-haiku-5-5',
    prompt_version: 1,
    max_prompt_length: 500,
    max_names: 30,
    default_names: 20,
    daily_limit: 20,
    used_today: 1,
    active_request_id: null,
    ...overrides,
  };
}

const candidates: AdminCurationCandidate[] = [
  {
    id: 'c1',
    position: 0,
    name: 'Иван Гончаров',
    note: '<b>Хьюго</b>',
    match_status: 'matched',
    options: [{ qid: 'Q192279', label: 'Иван Александрович Гончаров', description: 'русский писатель', existing: null }],
  },
  {
    id: 'c2',
    position: 1,
    name: 'Иван Петров',
    note: '',
    match_status: 'ambiguous',
    options: [
      { qid: 'Q1', label: 'Иван Петров', description: 'поэт', existing: null },
      { qid: 'Q2', label: 'Иван Петров', description: '', existing: null },
    ],
  },
  {
    id: 'c3',
    position: 2,
    name: 'Мусорный',
    note: '',
    match_status: 'matched',
    options: [
      {
        qid: 'Q3',
        label: 'Мусорный Автор',
        description: 'x',
        existing: { id: 'a3', name: 'Мусорный Автор', slug: 'm', status: 'rejected' },
      },
    ],
  },
  { id: 'c4', position: 3, name: 'Выдуманный', note: '', match_status: 'not_found', options: [] },
  { id: 'c5', position: 4, name: 'Сбой', note: '', match_status: 'error', options: [] },
  { id: 'c6', position: 5, name: 'Ждёт', note: '', match_status: 'pending', options: [] },
];

function request(overrides: Partial<AdminCurationRequest> = {}): AdminCurationRequest {
  return {
    id: 'r1',
    prompt: 'русская классика',
    max_names: 20,
    status: 'done',
    error_code: null,
    model: 'claude-haiku-5-5',
    prompt_version: 1,
    created_by: { id: 'u1', username: 'admin' },
    created_at: '2026-10-13T12:00:00Z',
    finished_at: '2026-10-13T12:01:00Z',
    candidates_total: 6,
    candidates_resolved: 5,
    input_tokens: 900,
    output_tokens: 2400,
    candidates,
    ...overrides,
  };
}

function author(overrides: Partial<AdminAuthor> = {}): AdminAuthor {
  return {
    id: 'a1',
    name: 'Иван Гончаров',
    slug: 'ivan-goncharov',
    biography: null,
    photo_url: null,
    birth_date: null,
    death_date: null,
    status: 'pending',
    external_source: 'wikidata',
    external_id: 'Q192279',
    photo_attribution: null,
    auto_harvest: false,
    last_auto_harvested_at: null,
    ...overrides,
  };
}

function renderPanel() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <AiAuthorCurationPanel />
    </QueryClientProvider>
  );
}

function checkboxes() {
  return screen.queryAllByRole('checkbox', { name: /^Выбрать / }) as HTMLInputElement[];
}

describe('AiAuthorCurationPanel', () => {
  beforeEach(() => {
    searchParamsValue = new URLSearchParams();
    statusMock.mockResolvedValue(status());
    listMock.mockResolvedValue({ items: [request()] });
    getMock.mockResolvedValue(request());
    vi.spyOn(window, 'confirm').mockReturnValue(true);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    statusMock.mockReset();
    listMock.mockReset();
    getMock.mockReset();
    importMock.mockReset();
    triggerMock.mockReset();
  });

  it('off state: explains it, has no submit and still lists recent requests', async () => {
    statusMock.mockResolvedValue(status({ enabled: false }));
    renderPanel();

    expect(await screen.findByText('Подбор ИИ выключен')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Подобрать авторов' })).not.toBeInTheDocument();
    expect(await screen.findByRole('link', { name: /русская классика/ })).toHaveAttribute(
      'href',
      '/import/authors/ai?request=r1'
    );
  });

  it('shows progress while running', async () => {
    searchParamsValue = new URLSearchParams({ request: 'r1' });
    getMock.mockResolvedValue(
      request({ status: 'running', finished_at: null, candidates_total: 3, candidates_resolved: 1, candidates: candidates.slice(0, 1) })
    );
    renderPanel();
    expect(await screen.findByText('Проверено в Wikidata: 1 из 3')).toBeInTheDocument();
  });

  it('renders match badges and notes as plain text, with nothing preselected', async () => {
    searchParamsValue = new URLSearchParams({ request: 'r1' });
    renderPanel();

    const items = await screen.findAllByTestId('curation-candidate');
    expect(items).toHaveLength(6);
    expect(within(items[0]).getByText('Найден')).toBeInTheDocument();
    expect(within(items[1]).getByText('Неоднозначно')).toBeInTheDocument();
    expect(within(items[3]).getByText('Не найден')).toBeInTheDocument();
    expect(within(items[4]).getByText('Ошибка Wikidata')).toBeInTheDocument();
    expect(within(items[5]).getByText('Проверяется…')).toBeInTheDocument();

    // The note is literal text: no <b> element is rendered.
    expect(within(items[0]).getByText('<b>Хьюго</b>')).toBeInTheDocument();
    expect(items[0].querySelector('b')).toBeNull();
    expect(within(items[0]).getByText('Мнение ИИ:')).toBeInTheDocument();

    expect(checkboxes().every((c) => !c.checked)).toBe(true);
    expect(screen.getByRole('button', { name: 'Импортировать выбранных' })).toBeDisabled();
  });

  it('rejected options are not selectable and offer no import', async () => {
    searchParamsValue = new URLSearchParams({ request: 'r1' });
    renderPanel();
    const items = await screen.findAllByTestId('curation-candidate');

    expect(within(items[2]).queryByRole('checkbox')).not.toBeInTheDocument();
    expect(within(items[2]).getByText('Отклонён ранее')).toBeInTheDocument();
    expect(within(items[2]).queryByRole('button', { name: /Импортировать/ })).not.toBeInTheDocument();
    // Q192279 + Q1 + Q2 are selectable.
    expect(checkboxes()).toHaveLength(3);
  });

  it('«Выбрать все найденные» ticks only new matched authors', async () => {
    searchParamsValue = new URLSearchParams({ request: 'r1' });
    renderPanel();
    await screen.findAllByTestId('curation-candidate');

    fireEvent.click(screen.getByRole('button', { name: 'Выбрать все найденные' }));
    const ticked = checkboxes().filter((c) => c.checked).map((c) => c.getAttribute('aria-label'));
    expect(ticked).toEqual(['Выбрать Иван Александрович Гончаров (Q192279)']);
    expect(screen.getByText('Выбрано: 1')).toBeInTheDocument();
  });

  it('per-option actions reuse the Wikidata author import', async () => {
    searchParamsValue = new URLSearchParams({ request: 'r1' });
    importMock.mockResolvedValue({
      outcome: 'created',
      match: null,
      qid_backfilled: false,
      namesake_of: null,
      enrichment: null,
      author: author(),
    });
    renderPanel();
    const items = await screen.findAllByTestId('curation-candidate');

    fireEvent.click(within(items[0]).getByRole('button', { name: 'Импортировать автора' }));
    await waitFor(() => expect(importMock).toHaveBeenCalledWith('Q192279'));
    expect(await screen.findByText(/импортирован и ждёт модерации/)).toBeInTheDocument();
    expect(triggerMock).not.toHaveBeenCalled();
  });

  it('bulk import with books queues one harvest for the ticked authors', async () => {
    searchParamsValue = new URLSearchParams({ request: 'r1' });
    importMock.mockImplementation(async (qid: string) => ({
      outcome: 'created',
      match: null,
      qid_backfilled: false,
      namesake_of: null,
      enrichment: null,
      author: author({ id: `id-${qid}`, external_id: qid, name: qid }),
    }));
    triggerMock.mockResolvedValue({ task_id: 't1' });
    renderPanel();
    await screen.findAllByTestId('curation-candidate');

    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать Иван Петров (Q1)' }));
    fireEvent.click(screen.getByRole('checkbox', { name: 'Выбрать Иван Петров (Q2)' }));
    fireEvent.click(screen.getByRole('button', { name: 'Импортировать выбранных с книгами' }));

    await waitFor(() => expect(triggerMock).toHaveBeenCalledWith({ author_qids: ['Q1', 'Q2'], limit: 600 }));
    expect(importMock.mock.calls.map((c) => c[0])).toEqual(['Q1', 'Q2']);
    expect(await screen.findByText(/Харвест книг запущен для авторов: 2/)).toBeInTheDocument();
  });

  it('a failed request shows a readable error', async () => {
    searchParamsValue = new URLSearchParams({ request: 'r1' });
    getMock.mockResolvedValue(
      request({ status: 'failed', error_code: 'model_refusal', candidates: [], candidates_total: 0, candidates_resolved: 0 })
    );
    renderPanel();
    expect(await screen.findByText(/Модель отказалась отвечать/)).toBeInTheDocument();
  });
});
