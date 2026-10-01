import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { NotificationBell } from './notification-bell';

const unreadCountMock = vi.fn();
const listMock = vi.fn();
const markReadMock = vi.fn();
const markAllReadMock = vi.fn();
const pushMock = vi.fn();
let isAuthenticatedValue = true;

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
}));

vi.mock('@/entities/session', () => ({
  useAuthStore: () => ({ isAuthenticated: isAuthenticatedValue }),
}));

// Forward only the real API's own parameters (not react-query v5's internal
// second `mutationFn` context argument) so mock call assertions match what
// the production api-client functions are actually invoked with.
vi.mock('@/entities/notification', () => ({
  notificationsApi: {
    unreadCount: () => unreadCountMock(),
    list: (params?: unknown) => listMock(params),
    markRead: (id: string) => markReadMock(id),
    markAllRead: () => markAllReadMock(),
  },
}));

function renderBell() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <NotificationBell />
    </QueryClientProvider>
  );
}

const sampleItem = {
  id: 'n1',
  type: 'review_like' as const,
  actor: { id: 'u1', name: 'Алиса', avatar_url: null },
  review: { id: 'r1', book_id: 'b1', book_slug: 'some-book', book_title: 'Какая-то книга' },
  is_read: false,
  created_at: new Date().toISOString(),
};

describe('NotificationBell', () => {
  beforeEach(() => {
    isAuthenticatedValue = true;
    unreadCountMock.mockReset();
    listMock.mockReset();
    markReadMock.mockReset();
    markAllReadMock.mockReset();
    pushMock.mockReset();
    unreadCountMock.mockResolvedValue({ count: 0 });
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });
    markAllReadMock.mockResolvedValue({ status: 'ok' });
    markReadMock.mockResolvedValue({ status: 'ok' });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders nothing for unauthenticated users', () => {
    isAuthenticatedValue = false;
    const { container } = renderBell();
    expect(container).toBeEmptyDOMElement();
  });

  it('hides the badge when there are no unread notifications', async () => {
    unreadCountMock.mockResolvedValue({ count: 0 });
    renderBell();

    await waitFor(() => expect(unreadCountMock).toHaveBeenCalled());
    expect(screen.queryByText('0')).not.toBeInTheDocument();
  });

  it('shows the unread count badge, capped at "9+"', async () => {
    unreadCountMock.mockResolvedValue({ count: 15 });
    renderBell();

    await waitFor(() => expect(screen.getByText('9+')).toBeInTheDocument());
  });

  it('shows the exact unread count under the cap', async () => {
    unreadCountMock.mockResolvedValue({ count: 3 });
    renderBell();

    await waitFor(() => expect(screen.getByText('3')).toBeInTheDocument());
  });

  it('loads the list on open and marks a notification read + navigates on click', async () => {
    unreadCountMock.mockResolvedValue({ count: 1 });
    listMock.mockResolvedValue({ items: [sampleItem], total: 1, page: 1, limit: 20 });

    renderBell();

    fireEvent.click(await screen.findByLabelText('Уведомления'));

    await waitFor(() => expect(listMock).toHaveBeenCalled());
    const item = await screen.findByText(/Какая-то книга/);
    fireEvent.click(item);

    await waitFor(() => expect(markReadMock).toHaveBeenCalledWith('n1'));
    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/books/some-book'));
  });

  it('shows the empty state when there are no notifications', async () => {
    unreadCountMock.mockResolvedValue({ count: 0 });
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, limit: 20 });

    renderBell();

    fireEvent.click(await screen.findByLabelText('Уведомления'));

    await waitFor(() => expect(screen.getByText('Уведомлений пока нет')).toBeInTheDocument());
  });

  it('triggers mark-all-read and invalidates the unread-count/list queries', async () => {
    unreadCountMock.mockResolvedValue({ count: 2 });
    listMock.mockResolvedValue({ items: [sampleItem], total: 1, page: 1, limit: 20 });

    renderBell();

    fireEvent.click(await screen.findByLabelText('Уведомления'));
    await waitFor(() => expect(listMock).toHaveBeenCalled());

    const initialUnreadCountCalls = unreadCountMock.mock.calls.length;
    const initialListCalls = listMock.mock.calls.length;

    fireEvent.click(screen.getByText('Отметить все прочитанными'));

    await waitFor(() => expect(markAllReadMock).toHaveBeenCalled());
    await waitFor(() =>
      expect(unreadCountMock.mock.calls.length).toBeGreaterThan(initialUnreadCountCalls)
    );
    await waitFor(() => expect(listMock.mock.calls.length).toBeGreaterThan(initialListCalls));
  });
});
