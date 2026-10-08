'use client';

import { useEffect, useState } from 'react';
import type { AdminUser, AdminUserRole } from '@bookhaven/api-client';
import { Select } from '@bookhaven/ui';
import { USER_ROLES, isUserRole, userRoleLabel } from '@/entities/user';
import { useChangeUserRole } from '../model/use-change-user-role';
import { changeRoleConfirmText, changeRoleErrorText } from '../lib/messages';

const ROLE_OPTIONS = USER_ROLES.map((role) => ({ value: role, label: userRoleLabel(role) }));

export interface UserRoleSelectProps {
  user: AdminUser;
  disabled?: boolean;
  onChanged?: (user: AdminUser, from: AdminUserRole, to: AdminUserRole) => void;
  onError?: (message: string) => void;
}

/**
 * Per-row role control: picking a different role asks window.confirm, then
 * sends the change. Cancel (or a failed request) reverts the select. The
 * select is disabled while its own request is pending.
 */
export function UserRoleSelect({ user, disabled, onChanged, onError }: UserRoleSelectProps) {
  const mutation = useChangeUserRole();
  // Role shown after a confirmed change until the refetched row arrives.
  const [shownRole, setShownRole] = useState<AdminUserRole | null>(null);

  useEffect(() => {
    setShownRole(null);
  }, [user.role]);

  const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const from = shownRole ?? user.role;
    const to = e.target.value;
    if (!isUserRole(to) || to === from) return;
    if (!window.confirm(changeRoleConfirmText(user, from, to))) {
      // Controlled select: not updating state reverts the DOM value.
      return;
    }
    setShownRole(to);
    mutation.mutate(
      { id: user.id, role: to },
      {
        onSuccess: (updated) => onChanged?.(updated, from, to),
        onError: (error) => {
          setShownRole(null);
          onError?.(changeRoleErrorText(error));
        },
      }
    );
  };

  return (
    <Select
      aria-label={`Роль пользователя ${user.email}`}
      options={ROLE_OPTIONS}
      value={shownRole ?? user.role}
      onChange={handleChange}
      disabled={disabled || mutation.isPending}
      className="w-full md:w-48"
    />
  );
}
