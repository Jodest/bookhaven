import { api } from '@bookhaven/api-client';

export const notificationsApi = {
  list: api.notifications.list,
  unreadCount: api.notifications.unreadCount,
  markRead: api.notifications.markRead,
  markAllRead: api.notifications.markAllRead,
};
