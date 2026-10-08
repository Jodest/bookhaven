import { ApiError } from '@bookhaven/api-client';
import type { AdminUser, AdminUserRole } from '@bookhaven/api-client';
import { userRoleLabel } from '@/entities/user';

/** Display name of a user in confirm/feedback texts: name, else email. */
export function userDisplayName(user: Pick<AdminUser, 'name' | 'email'>): string {
  return user.name?.trim() || user.email;
}

/** window.confirm text before a role change. */
export function changeRoleConfirmText(
  user: Pick<AdminUser, 'name' | 'email'>,
  from: AdminUserRole,
  to: AdminUserRole
): string {
  let text = `Изменить роль пользователя «${userDisplayName(user)}»: ${userRoleLabel(from)} → ${userRoleLabel(to)}?`;
  if (to === 'admin') {
    text += ' Пользователь получит полный доступ к админ-панели.';
  }
  return text;
}

/** Success banner text after a role change. */
export function changeRoleSuccessText(
  user: Pick<AdminUser, 'name' | 'email'>,
  from: AdminUserRole,
  to: AdminUserRole
): string {
  return `Роль пользователя «${userDisplayName(user)}» изменена: ${userRoleLabel(from)} → ${userRoleLabel(to)}.`;
}

const ERROR_TEXTS: Record<string, string> = {
  last_admin: 'Нельзя снять роль с последнего администратора.',
  cannot_change_own_role: 'Нельзя изменить собственную роль.',
  user_not_found: 'Пользователь не найден — обновите список.',
  invalid_role: 'Недопустимая роль.',
};

/** Maps a role-change failure (ApiError.code) to a user-facing message. */
export function changeRoleErrorText(error: unknown): string {
  if (error instanceof ApiError && error.code && ERROR_TEXTS[error.code]) {
    return ERROR_TEXTS[error.code];
  }
  return 'Не удалось изменить роль. Попробуйте ещё раз.';
}
