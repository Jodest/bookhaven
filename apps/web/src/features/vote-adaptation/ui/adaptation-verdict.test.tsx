import type { AdaptationVerdict as Verdict, AdaptationVerdicts } from '@bookhaven/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render as rtlRender, screen, waitFor } from '@testing-library/react';
import type { ReactElement } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AdaptationVerdict } from './adaptation-verdict';

let isAuthenticatedValue = true;
let queryClient: QueryClient;
const verdictsMock = vi.fn();
const voteMock = vi.fn();
const unvoteMock = vi.fn();

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ isAuthenticated: isAuthenticatedValue, user: { id: 'u1' } }),
}));

vi.mock('@/entities/screen-work', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/screen-work')>()),
  screenWorksApi: {
    verdicts: (bookId: string) => verdictsMock(bookId),
    vote: (bookId: string, workId: string, value: string) => voteMock(bookId, workId, value),
    unvote: (bookId: string, workId: string) => unvoteMock(bookId, workId),
  },
}));

function render(ui: ReactElement) {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return rtlRender(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

function item(over: Partial<Verdict> = {}): Verdict {
  return {
    screen_work_id: 'w1',
    votes_count: 23,
    book_better: 18,
    adaptation_better: 3,
    same: 2,
    my_vote: null,
    ...over,
  };
}

function verdicts(over: Partial<AdaptationVerdicts> = {}): AdaptationVerdicts {
  return { can_vote: true, min_votes: 1, items: [item()], ...over };
}

const ui = <AdaptationVerdict bookId="b1" screenWorkId="w1" />;

describe('AdaptationVerdict', () => {
  beforeEach(() => {
    isAuthenticatedValue = true;
  });
  afterEach(() => {
    verdictsMock.mockReset();
    voteMock.mockReset();
    unvoteMock.mockReset();
  });

  it('shows only the verdict line to a guest, without prompt or buttons', async () => {
    isAuthenticatedValue = false;
    verdictsMock.mockResolvedValue(verdicts({ can_vote: false }));
    render(ui);

    expect(await screen.findByText('78% читателей: книга лучше · 23 голоса')).toBeInTheDocument();
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('shows only the verdict line to a reader without the book on «Прочитано»', async () => {
    verdictsMock.mockResolvedValue(verdicts({ can_vote: false }));
    render(ui);

    expect(await screen.findByText('78% читателей: книга лучше · 23 голоса')).toBeInTheDocument();
    expect(screen.queryByText(/Отметьте книгу/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('shows a compact dropdown with the current vote', async () => {
    verdictsMock.mockResolvedValue(verdicts({ items: [item({ my_vote: 'same' })] }));
    render(ui);

    const select = (await screen.findByLabelText('Что лучше:')) as HTMLSelectElement;
    expect(screen.queryByText(/Голосуя/)).not.toBeInTheDocument();
    expect(select.value).toBe('same');
    expect(Array.from(select.options).map((o) => o.textContent)).toEqual([
      'Книга',
      'Экранизация',
      'Одинаково',
      'Убрать голос',
    ]);
  });

  it('offers «Не выбрано» and no «Убрать голос» without a vote', async () => {
    verdictsMock.mockResolvedValue(verdicts());
    render(ui);

    const select = (await screen.findByLabelText('Что лучше:')) as HTMLSelectElement;
    expect(select.value).toBe('');
    expect(Array.from(select.options).map((o) => o.textContent)).toEqual([
      'Не выбрано',
      'Книга',
      'Экранизация',
      'Одинаково',
    ]);
  });

  it('has no divider line above the verdict', async () => {
    verdictsMock.mockResolvedValue(verdicts());
    const { container } = render(ui);
    await screen.findByLabelText('Что лучше:');
    expect(container.querySelector('.border-t')).toBeNull();
  });

  it('shows no verdict line with no votes, but keeps the dropdown', async () => {
    verdictsMock.mockResolvedValue(
      verdicts({ items: [item({ votes_count: 0, book_better: null, adaptation_better: null, same: null })] }),
    );
    render(ui);

    expect(await screen.findByLabelText('Что лучше:')).toBeInTheDocument();
    expect(screen.queryByText(/читателей/)).not.toBeInTheDocument();
    expect(screen.queryByText('Пока мало голосов')).not.toBeInTheDocument();
  });

  it('shows the verdict of a single vote to a guest', async () => {
    isAuthenticatedValue = false;
    verdictsMock.mockResolvedValue(
      verdicts({
        can_vote: false,
        items: [item({ votes_count: 1, book_better: 1, adaptation_better: 0, same: 0 })],
      }),
    );
    render(ui);

    expect(await screen.findByText('100% читателей: книга лучше · 1 голос')).toBeInTheDocument();
  });

  it('renders nothing for a guest when there are no votes', async () => {
    isAuthenticatedValue = false;
    verdictsMock.mockResolvedValue(
      verdicts({
        can_vote: false,
        items: [item({ votes_count: 0, book_better: null, adaptation_better: null, same: null })],
      }),
    );
    const { container } = render(ui);
    await waitFor(() => expect(verdictsMock).toHaveBeenCalled());
    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });

  it('votes and updates the verdict line from the response', async () => {
    verdictsMock.mockResolvedValue(
      verdicts({ items: [item({ votes_count: 0, book_better: null, adaptation_better: null, same: null })] }),
    );
    voteMock.mockResolvedValue(
      item({ my_vote: 'adaptation', votes_count: 1, book_better: 0, adaptation_better: 1, same: 0 }),
    );
    render(ui);

    fireEvent.change(await screen.findByLabelText('Что лучше:'), { target: { value: 'adaptation' } });

    expect(await screen.findByText('100% читателей: экранизация лучше · 1 голос')).toBeInTheDocument();
    expect(voteMock).toHaveBeenCalledWith('b1', 'w1', 'adaptation');
    expect((screen.getByLabelText('Что лучше:') as HTMLSelectElement).value).toBe('adaptation');
  });

  it('changes the vote to another value', async () => {
    verdictsMock.mockResolvedValue(verdicts({ items: [item({ my_vote: 'book' })] }));
    voteMock.mockResolvedValue(item({ my_vote: 'same' }));
    render(ui);

    fireEvent.change(await screen.findByLabelText('Что лучше:'), { target: { value: 'same' } });

    await waitFor(() => expect(voteMock).toHaveBeenCalledWith('b1', 'w1', 'same'));
    await waitFor(() => expect((screen.getByLabelText('Что лучше:') as HTMLSelectElement).value).toBe('same'));
  });

  it('removes the vote with «Убрать голос» and falls back to «Не выбрано»', async () => {
    verdictsMock
      .mockResolvedValueOnce(verdicts({ items: [item({ my_vote: 'adaptation' })] }))
      .mockResolvedValue(verdicts());
    unvoteMock.mockResolvedValue(undefined);
    render(ui);

    fireEvent.change(await screen.findByLabelText('Что лучше:'), { target: { value: 'remove' } });

    await waitFor(() => expect(unvoteMock).toHaveBeenCalledWith('b1', 'w1'));
    await waitFor(() => {
      const select = screen.getByLabelText('Что лучше:') as HTMLSelectElement;
      expect(select.value).toBe('');
      expect(Array.from(select.options).map((o) => o.textContent)).not.toContain('Убрать голос');
    });
  });

  it('disables the dropdown while a request runs', async () => {
    verdictsMock.mockResolvedValue(verdicts());
    let resolveVote: (v: Verdict) => void = () => {};
    voteMock.mockReturnValue(new Promise<Verdict>((resolve) => (resolveVote = resolve)));
    render(ui);

    fireEvent.change(await screen.findByLabelText('Что лучше:'), { target: { value: 'book' } });
    await waitFor(() => expect(screen.getByLabelText('Что лучше:')).toBeDisabled());
    resolveVote(item({ my_vote: 'book' }));
    await waitFor(() => expect(screen.getByLabelText('Что лучше:')).not.toBeDisabled());
  });

  it('keeps the vote of a reader who left «Прочитано» and lets them remove it', async () => {
    verdictsMock
      .mockResolvedValueOnce(verdicts({ can_vote: false, items: [item({ my_vote: 'book' })] }))
      .mockResolvedValue(verdicts({ can_vote: false }));
    unvoteMock.mockResolvedValue(undefined);
    render(ui);

    expect(await screen.findByText('Ваш голос: Книга лучше')).toBeInTheDocument();
    expect(screen.queryByLabelText('Что лучше:')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Убрать голос' }));

    await waitFor(() => expect(unvoteMock).toHaveBeenCalledWith('b1', 'w1'));
    await waitFor(() => expect(screen.queryByText('Ваш голос: Книга лучше')).not.toBeInTheDocument());
  });

  it('shows an error and restores the previous value when saving fails', async () => {
    verdictsMock.mockResolvedValue(verdicts({ items: [item({ my_vote: 'book' })] }));
    voteMock.mockRejectedValue(new Error('boom'));
    render(ui);

    fireEvent.change(await screen.findByLabelText('Что лучше:'), { target: { value: 'same' } });

    expect(await screen.findByText('Не удалось сохранить голос.')).toBeInTheDocument();
    await waitFor(() => expect((screen.getByLabelText('Что лучше:') as HTMLSelectElement).value).toBe('book'));
  });

  it('renders nothing when the verdicts fail to load or the adaptation has no item', async () => {
    verdictsMock.mockRejectedValue(new Error('boom'));
    const { container, unmount } = render(ui);
    await waitFor(() => expect(verdictsMock).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
    unmount();

    verdictsMock.mockResolvedValue(verdicts({ items: [item({ screen_work_id: 'other' })] }));
    const second = render(ui);
    await waitFor(() => expect(verdictsMock).toHaveBeenCalledTimes(2));
    expect(second.container).toBeEmptyDOMElement();
  });
});
