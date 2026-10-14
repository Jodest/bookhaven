import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '@bookhaven/api-client';
import type { AdminCurationStatus } from '@bookhaven/api-client';
import { CurationPromptForm } from './curation-prompt-form';

const createMock = vi.fn();

vi.mock('@/entities/ai-curation', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/ai-curation')>()),
  adminAiCuratorApi: {
    create: (...args: unknown[]) => createMock(...args),
  },
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
    used_today: 3,
    active_request_id: null,
    ...overrides,
  };
}

function renderForm(st: AdminCurationStatus, onCreated = vi.fn()) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <CurationPromptForm status={st} onCreated={onCreated} />
    </QueryClientProvider>
  );
  return { onCreated };
}

const submit = () => screen.getByRole('button', { name: 'Подобрать авторов' });
const promptField = () => screen.getByLabelText('Каких авторов не хватает в каталоге?');

describe('CurationPromptForm', () => {
  beforeEach(() => {
    createMock.mockReset();
  });

  it('off state: says so and offers no submit', () => {
    renderForm(status({ enabled: false }));
    expect(screen.getByText('Подбор ИИ выключен')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Подобрать авторов' })).not.toBeInTheDocument();
  });

  it('shows a counter, caps the prompt at 500 and defaults to 20 authors', () => {
    renderForm(status());
    expect(screen.getByText('0 / 500')).toBeInTheDocument();
    expect(promptField()).toHaveAttribute('maxLength', '500');
    expect(screen.getByLabelText('Сколько авторов')).toHaveValue(20);
    expect(submit()).toBeDisabled();

    fireEvent.change(promptField(), { target: { value: 'русская классика' } });
    expect(screen.getByText('16 / 500')).toBeInTheDocument();
    expect(submit()).toBeEnabled();

    fireEvent.change(screen.getByLabelText('Сколько авторов'), { target: { value: '31' } });
    expect(submit()).toBeDisabled();
  });

  it('submit is disabled while a request is active', () => {
    renderForm(status({ active_request_id: 'r1' }));
    fireEvent.change(promptField(), { target: { value: 'классика' } });
    expect(submit()).toBeDisabled();
    expect(screen.getByText(/ещё выполняется/)).toBeInTheDocument();
  });

  it('submits the trimmed prompt and the count', async () => {
    createMock.mockResolvedValue({ id: 'new-id' });
    const { onCreated } = renderForm(status());
    fireEvent.change(promptField(), { target: { value: '  лауреаты Хьюго  ' } });
    fireEvent.change(screen.getByLabelText('Сколько авторов'), { target: { value: '12' } });
    fireEvent.click(submit());
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('new-id'));
    expect(createMock).toHaveBeenCalledWith({ prompt: 'лауреаты Хьюго', max_names: 12 });
  });

  it.each([
    [409, 'У вас уже есть подборка в работе'],
    [429, 'Достигнут дневной лимит'],
    [503, 'Подбор ИИ выключен'],
    [502, 'Не удалось поставить подборку в очередь'],
  ] as const)('maps %d to a readable message', async (code, text) => {
    createMock.mockRejectedValue(new ApiError(code, 'x', 'raw server text'));
    renderForm(status());
    fireEvent.change(promptField(), { target: { value: 'классика' } });
    fireEvent.click(submit());
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(text));
  });
});
