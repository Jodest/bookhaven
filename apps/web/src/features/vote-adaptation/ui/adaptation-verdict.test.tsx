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
  return { can_vote: true, min_votes: 5, items: [item()], ...over };
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

  it('shows the segmented buttons with the current vote selected and no note', async () => {
    verdictsMock.mockResolvedValue(verdicts({ items: [item({ my_vote: 'same' })] }));
    render(ui);

    expect(await screen.findByText('Что лучше:')).toBeInTheDocument();
    expect(screen.queryByText(/Голосуя/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Одинаково' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'Книга' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: 'Экранизация' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('votes and updates the verdict line from the response', async () => {
    verdictsMock.mockResolvedValue(
      verdicts({ items: [item({ votes_count: 3, book_better: null, adaptation_better: null, same: null })] }),
    );
    voteMock.mockResolvedValue(
      item({ my_vote: 'adaptation', votes_count: 6, book_better: 1, adaptation_better: 4, same: 1 }),
    );
    render(ui);

    expect(await screen.findByText('Пока мало голосов')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Экранизация' }));

    expect(await screen.findByText('67% читателей: экранизация лучше · 6 голосов')).toBeInTheDocument();
    expect(voteMock).toHaveBeenCalledWith('b1', 'w1', 'adaptation');
    expect(screen.getByRole('button', { name: 'Экранизация' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('removes the vote when the selected button is clicked', async () => {
    verdictsMock
      .mockResolvedValueOnce(verdicts({ items: [item({ my_vote: 'adaptation' })] }))
      .mockResolvedValue(verdicts());
    unvoteMock.mockResolvedValue(undefined);
    render(ui);

    fireEvent.click(await screen.findByRole('button', { name: 'Экранизация' }));

    await waitFor(() => expect(unvoteMock).toHaveBeenCalledWith('b1', 'w1'));
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Экранизация' })).toHaveAttribute('aria-pressed', 'false'),
    );
  });

  it('keeps the vote of a reader who left «Прочитано» and lets them remove it', async () => {
    verdictsMock
      .mockResolvedValueOnce(verdicts({ can_vote: false, items: [item({ my_vote: 'book' })] }))
      .mockResolvedValue(verdicts({ can_vote: false }));
    unvoteMock.mockResolvedValue(undefined);
    render(ui);

    expect(await screen.findByText('Ваш голос: Книга лучше')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Экранизация' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Убрать голос' }));

    await waitFor(() => expect(unvoteMock).toHaveBeenCalledWith('b1', 'w1'));
    await waitFor(() => expect(screen.queryByText('Ваш голос: Книга лучше')).not.toBeInTheDocument());
  });

  it('shows an error and keeps the previous state when saving fails', async () => {
    verdictsMock.mockResolvedValue(verdicts({ items: [item({ my_vote: 'book' })] }));
    voteMock.mockRejectedValue(new Error('boom'));
    render(ui);

    fireEvent.click(await screen.findByRole('button', { name: 'Одинаково' }));

    expect(await screen.findByText('Не удалось сохранить голос.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Книга' })).toHaveAttribute('aria-pressed', 'true');
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
