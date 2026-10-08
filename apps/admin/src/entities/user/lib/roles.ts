import type { AdminUserRole } from '@bookhaven/api-client';

/** Every user_role value, from least to most privileged. */
export const USER_ROLES: readonly AdminUserRole[] = ['user', 'contributor', 'moderator', 'admin'];

const USER_ROLE_LABELS: Record<AdminUserRole, string> = {
  user: 'Читатель',
  contributor: 'Автор контента',
  moderator: 'Модератор',
  admin: 'Администратор',
};

/** Russian label of a role (shared by the role filter and the row control). */
export function userRoleLabel(role: AdminUserRole): string {
  return USER_ROLE_LABELS[role] ?? role;
}

/** Type guard for a role coming from the URL or a <select>. */
export function isUserRole(value: string | null | undefined): value is AdminUserRole {
  return !!value && (USER_ROLES as readonly string[]).includes(value);
}
