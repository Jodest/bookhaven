import type { AdaptationVerdicts } from '@bookhaven/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render as rtlRender, screen, waitFor } from '@testing-library/react';
import type { ReactElement } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AdaptationVotePrompt } from './adaptation-vote-prompt';

let isAuthenticatedValue = true;
const verdictsMock = vi.fn();

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ isAuthenticated: isAuthenticatedValue, user: { id: 'u1' } }),
}));

vi.mock('@/entities/screen-work', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/screen-work')>()),
  screenWorksApi: { verdicts: (bookId: string) => verdictsMock(bookId) },
}));

function render(ui: ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return rtlRender(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

function verdicts(over: Partial<AdaptationVerdicts> = {}): AdaptationVerdicts {
  return { can_vote: true, min_votes: 5, items: [], ...over };
}

const ui = <AdaptationVotePrompt bookId="b1" slug="book-1" />;

describe('AdaptationVotePrompt', () => {
  beforeEach(() => {
    isAuthenticatedValue = true;
  });
  afterEach(() => verdictsMock.mockReset());

  it('shows a login link to a guest', async () => {
    isAuthenticatedValue = false;
    verdictsMock.mockResolvedValue(verdicts({ can_vote: false }));
    render(ui);

    const link = await screen.findByRole('link', { name: 'Войдите, чтобы сравнить книгу и экранизацию' });
    expect(link.getAttribute('href')).toContain('/login');
  });

  it('asks a reader who cannot vote to mark the book as read', async () => {
    verdictsMock.mockResolvedValue(verdicts({ can_vote: false }));
    render(ui);

    expect(
      await screen.findByText('Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.'),
    ).toBeInTheDocument();
  });

  it('renders nothing for a reader who can vote', async () => {
    verdictsMock.mockResolvedValue(verdicts());
    const { container } = render(ui);
    await waitFor(() => expect(verdictsMock).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing when the verdicts fail to load', async () => {
    verdictsMock.mockRejectedValue(new Error('boom'));
    const { container } = render(ui);
    await waitFor(() => expect(verdictsMock).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });
});
