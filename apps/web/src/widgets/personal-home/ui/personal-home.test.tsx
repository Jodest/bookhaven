import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { Book, PersonalHome as PersonalHomeData, ReadingChallenge } from '@bookhaven/api-client';
import type { ImgHTMLAttributes } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ChallengeProgress } from './challenge-progress';
import { PersonalHome } from './personal-home';
import { PersonalRecommendations } from './personal-recommendations';
import { WantToReadPick } from './want-to-read-pick';

const meMock = vi.fn();
const challengeMock = vi.fn();
const authState = { user: null as { id: string } | null, isAuthenticated: false, isLoading: false };

vi.mock('next/image', () => ({
  // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
  default: ({ fill: _fill, ...props }: ImgHTMLAttributes<HTMLImageElement> & { fill?: boolean }) => <img {...props} />,
}));

vi.mock('@/entities/session', () => ({
  useAuthStore: () => authState,
}));

vi.mock('@/entities/home', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/home')>()),
  homeApi: { me: () => meMock() },
}));

vi.mock('@/entities/reading-challenge', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/entities/reading-challenge')>()),
  readingChallengesApi: { get: (year: number) => challengeMock(year) },
}));

function makeBook(n: number): Book {
  return {
    id: `b${n}`,
    title: `Книга ${n}`,
    slug: `book-${n}`,
    description: null,
    coverUrl: null,
    isbn: null,
    publishedYear: 2000,
    pageCount: null,
    language: '',
    rating: 4.5,
    ratingsCount: 3,
    reviewsCount: 0,
    authors: [],
    genres: [],
    createdAt: '',
    updatedAt: '',
  };
}

const year = new Date().getFullYear();

function challenge(overrides: Partial<ReadingChallenge> = {}): ReadingChallenge {
  return { year, goal: 20, booksRead: 5, progressPercent: 25, completed: false, ...overrides };
}

function renderWidgets() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PersonalHome />
      <ChallengeProgress />
      <WantToReadPick />
      <PersonalRecommendations />
    </QueryClientProvider>
  );
}

function loggedIn() {
  authState.user = { id: 'u1' };
  authState.isAuthenticated = true;
  authState.isLoading = false;
}

describe('PersonalHome', () => {
  beforeEach(() => {
    authState.user = null;
    authState.isAuthenticated = false;
    authState.isLoading = false;
  });

  afterEach(() => {
    meMock.mockReset();
    challengeMock.mockReset();
  });

  it('renders nothing for a guest and makes no API calls', () => {
    const { container } = renderWidgets();
    expect(container).toBeEmptyDOMElement();
    expect(meMock).not.toHaveBeenCalled();
    expect(challengeMock).not.toHaveBeenCalled();
  });

  it('renders nothing while the session is resolving', () => {
    authState.isLoading = true;
    authState.isAuthenticated = false;
    const { container } = renderWidgets();
    expect(container).toBeEmptyDOMElement();
    expect(meMock).not.toHaveBeenCalled();
  });

  it('shows skeletons, then every personal block', async () => {
    loggedIn();
    let resolveMe: (v: PersonalHomeData) => void = () => {};
    meMock.mockReturnValue(new Promise<PersonalHomeData>((r) => (resolveMe = r)));
    challengeMock.mockResolvedValue(challenge());

    renderWidgets();
    expect(screen.getAllByTestId('personal-home-skeleton').length).toBeGreaterThan(0);

    resolveMe({
      continueReading: { shelfId: 's-reading', total: 7, books: [makeBook(1), makeBook(2)] },
      recommendations: [makeBook(3)],
      wantToReadPick: { shelfId: 's-wtr', total: 14, book: makeBook(4) },
    });

    expect(await screen.findByRole('link', { name: 'Все (7)' })).toHaveAttribute('href', '/shelves/s-reading');
    expect(screen.getByRole('heading', { name: 'Продолжить чтение' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Вам может понравиться' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Из вашего «Хочу прочитать»' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Книга 4' })).toHaveAttribute('href', '/books/book-4');
    expect(screen.getByRole('link', { name: 'Вся полка (14)' })).toHaveAttribute('href', '/shelves/s-wtr');
    expect(await screen.findByText(`Прочитано 5 из 20 в ${year}`)).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '25');
    expect(screen.queryByTestId('personal-home-skeleton')).not.toBeInTheDocument();
    expect(meMock).toHaveBeenCalledTimes(1); // one /home/me request for both widgets
    expect(challengeMock).toHaveBeenCalledWith(year);
  });

  it('hides empty blocks', async () => {
    loggedIn();
    meMock.mockResolvedValue({
      continueReading: { shelfId: 's-reading', total: 0, books: [] },
      recommendations: [],
      wantToReadPick: { shelfId: 's-wtr', total: 0, book: null },
    });
    challengeMock.mockRejectedValue(new Error('boom'));

    const { container } = renderWidgets();
    await waitFor(() => expect(container).toBeEmptyDOMElement());
    expect(screen.queryByRole('heading')).not.toBeInTheDocument();
  });

  it('shows the set-goal CTA when the goal is null', async () => {
    loggedIn();
    meMock.mockResolvedValue({ continueReading: null, recommendations: [], wantToReadPick: null });
    challengeMock.mockResolvedValue(challenge({ goal: null, booksRead: 2, progressPercent: null }));

    renderWidgets();
    const cta = await screen.findByRole('link', { name: 'Поставить цель' });
    expect(cta).toHaveAttribute('href', '/challenge');
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Продолжить чтение' })).not.toBeInTheDocument();
  });

  it('hides the personal blocks when /home/me fails', async () => {
    loggedIn();
    meMock.mockRejectedValue(new Error('boom'));
    challengeMock.mockResolvedValue(challenge());

    renderWidgets();
    expect(await screen.findByRole('heading', { name: 'Читательский вызов' })).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByTestId('personal-home-skeleton')).not.toBeInTheDocument());
    expect(screen.queryByRole('heading', { name: 'Продолжить чтение' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Вам может понравиться' })).not.toBeInTheDocument();
  });
});

describe('standalone widgets', () => {
  beforeEach(() => loggedIn());

  afterEach(() => {
    meMock.mockReset();
    challengeMock.mockReset();
  });

  it('ChallengeProgress alone does not call /home/me', async () => {
    challengeMock.mockResolvedValue(challenge());
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <ChallengeProgress />
      </QueryClientProvider>
    );
    expect(await screen.findByRole('heading', { name: 'Читательский вызов' })).toBeInTheDocument();
    expect(meMock).not.toHaveBeenCalled();
  });

  it('WantToReadPick alone renders only the want-to-read card', async () => {
    meMock.mockResolvedValue({
      continueReading: { shelfId: 's', total: 1, books: [makeBook(1)] },
      recommendations: [],
      wantToReadPick: { shelfId: 's-wtr', total: 3, book: makeBook(4) },
    });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <WantToReadPick />
      </QueryClientProvider>
    );
    expect(await screen.findByRole('link', { name: 'Вся полка (3)' })).toHaveAttribute('href', '/shelves/s-wtr');
    expect(screen.queryByRole('heading', { name: 'Продолжить чтение' })).not.toBeInTheDocument();
    expect(challengeMock).not.toHaveBeenCalled();
  });
});
