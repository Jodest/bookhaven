'use client';

import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { AdminUserRole } from '@bookhaven/api-client';
import { adminUsersApi, ADMIN_USERS_QUERY_KEY } from '@/entities/user';

export interface UserRoleChange {
  id: string;
  role: AdminUserRole;
}

/**
 * Changes a user's role via PATCH /admin/users/:id/role and invalidates
 * every ['admin','users'] list query on success.
 */
export function useChangeUserRole() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, role }: UserRoleChange) => adminUsersApi.changeRole(id, role),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ADMIN_USERS_QUERY_KEY });
    },
  });
}
