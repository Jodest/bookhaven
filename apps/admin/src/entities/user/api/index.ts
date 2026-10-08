import { api } from '@bookhaven/api-client';

export const adminUsersApi = {
  list: api.admin.users.list,
  changeRole: api.admin.users.changeRole,
};

/** Query-key prefix of every admin users query (list pages). */
export const ADMIN_USERS_QUERY_KEY = ['admin', 'users'] as const;
