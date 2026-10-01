'use client';

import { notificationsApi } from '@/entities/notification';
import { formatRelativeTime } from '@/shared/lib/format-relative-time';
import { useAuthStore } from '@/entities/session';
import type { Notification } from '@bookhaven/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Bell } from 'lucide-react';
import { useRouter } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';

const UNREAD_COUNT_QUERY_KEY = ['notifications', 'unread-count'];
const LIST_QUERY_KEY = ['notifications', 'list'];

// 30s, not harvest-progress's 5s: a liked-review notification is a
// non-urgent social signal, not active job progress (see
// docs/features/review-like-notifications.md UX).
const UNREAD_COUNT_POLL_INTERVAL_MS = 30_000;

function notificationMessage(notification: Notification): string {
  const actorName = notification.actor.name || 'Пользователь';
  const bookTitle = notification.review?.book_title;
  if (notification.type === 'review_like' && bookTitle) {
    // "понравилась" agrees with "рецензия" (feminine), not the actor, so
    // this phrasing needs no actor-gender handling.
    return `${actorName}: понравилась ваша рецензия на «${bookTitle}»`;
  }
  return `${actorName}: новое уведомление`;
}

// Same initials-fallback avatar pattern as entities/review's ReviewCard.
function ActorAvatar({ name, avatarUrl }: { name: string | null; avatarUrl: string | null }) {
  const initial = name ? name.charAt(0).toUpperCase() : '?';
  return (
    <div className="flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted text-xs font-medium">
      {avatarUrl ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={avatarUrl} alt={name ?? ''} className="h-full w-full object-cover" />
      ) : (
        initial
      )}
    </div>
  );
}

function NotificationItem({
  notification,
  onOpen,
}: {
  notification: Notification;
  onOpen: (notification: Notification) => void;
}) {
  return (
    <li>
      <button
        type="button"
        onClick={() => onOpen(notification)}
        className={`flex w-full items-start gap-3 px-4 py-3 text-left text-sm transition-colors hover:bg-accent ${
          notification.is_read ? 'text-muted-foreground' : 'bg-accent/40 font-medium text-foreground'
        }`}
      >
        <ActorAvatar name={notification.actor.name} avatarUrl={notification.actor.avatar_url} />
        <div>
          <div>{notificationMessage(notification)}</div>
          <div className="mt-1 text-xs text-muted-foreground">
            {formatRelativeTime(notification.created_at)}
          </div>
        </div>
      </button>
    </li>
  );
}

export function NotificationBell() {
  const { isAuthenticated } = useAuthStore();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) return;

    function handleClickOutside(event: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [isOpen]);

  const unreadCountQuery = useQuery({
    queryKey: UNREAD_COUNT_QUERY_KEY,
    queryFn: notificationsApi.unreadCount,
    enabled: isAuthenticated,
    refetchInterval: UNREAD_COUNT_POLL_INTERVAL_MS,
  });

  const listQuery = useQuery({
    queryKey: LIST_QUERY_KEY,
    queryFn: () => notificationsApi.list({ limit: 20 }),
    enabled: isAuthenticated && isOpen,
  });

  const markReadMutation = useMutation({
    mutationFn: notificationsApi.markRead,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: LIST_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: UNREAD_COUNT_QUERY_KEY });
    },
  });

  const markAllReadMutation = useMutation({
    mutationFn: notificationsApi.markAllRead,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: LIST_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: UNREAD_COUNT_QUERY_KEY });
    },
  });

  if (!isAuthenticated) {
    return null;
  }

  const unreadCount = unreadCountQuery.data?.count ?? 0;
  const items = listQuery.data?.items ?? [];

  const handleOpenNotification = (notification: Notification) => {
    setIsOpen(false);
    if (!notification.is_read) {
      markReadMutation.mutate(notification.id);
    }
    if (notification.review?.book_slug) {
      router.push(`/books/${notification.review.book_slug}` as any);
    }
  };

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setIsOpen((prev) => !prev)}
        className="relative inline-flex h-9 w-9 items-center justify-center rounded-md text-background transition-colors hover:bg-background/10"
        aria-label="Уведомления"
      >
        <Bell className="h-5 w-5" />
        {unreadCount > 0 && (
          <span className="absolute -right-1 -top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-semibold leading-none text-destructive-foreground">
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        )}
      </button>

      {isOpen && (
        <div className="absolute right-0 z-20 mt-2 w-80 overflow-hidden rounded-md border bg-popover text-popover-foreground shadow-md">
          <div className="max-h-96 overflow-y-auto">
            {items.length === 0 ? (
              <p className="px-4 py-6 text-center text-sm text-muted-foreground">
                Уведомлений пока нет
              </p>
            ) : (
              <ul className="divide-y">
                {items.map((notification) => (
                  <NotificationItem
                    key={notification.id}
                    notification={notification}
                    onOpen={handleOpenNotification}
                  />
                ))}
              </ul>
            )}
          </div>
          <div className="border-t px-4 py-2 text-right">
            <button
              type="button"
              onClick={() => markAllReadMutation.mutate()}
              disabled={markAllReadMutation.isPending || unreadCount === 0}
              className="text-xs font-medium text-primary hover:underline disabled:cursor-not-allowed disabled:opacity-50"
            >
              Отметить все прочитанными
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
