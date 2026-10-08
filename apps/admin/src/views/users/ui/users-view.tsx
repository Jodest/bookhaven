'use client';

import { useState } from 'react';
import { useSearchParams, useRouter, usePathname } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import type { AdminUser, AdminUserRole } from '@bookhaven/api-client';
import {
  Input,
  Button,
  Select,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@bookhaven/ui';
import { Search } from 'lucide-react';
import { Pagination } from '@/shared/ui';
import { useAuthStore } from '@/entities/session';
import {
  adminUsersApi,
  ADMIN_USERS_QUERY_KEY,
  USER_ROLES,
  isUserRole,
  userRoleLabel,
} from '@/entities/user';
import { UserRoleSelect, changeRoleSuccessText } from '@/features/change-user-role';

const ROLE_FILTER_OPTIONS = [
  { value: '', label: 'Все роли' },
  ...USER_ROLES.map((role) => ({ value: role, label: userRoleLabel(role) })),
];

const dateFormatter = new Intl.DateTimeFormat('ru-RU', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
});

function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : dateFormatter.format(date);
}

type Banner = { type: 'success' | 'error'; message: string };

export function UsersView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const currentUserId = useAuthStore((s) => s.user?.id);

  const [searchInput, setSearchInput] = useState(searchParams.get('search') || '');
  const [banner, setBanner] = useState<Banner | null>(null);

  const search = searchParams.get('search') || '';
  const roleParam = searchParams.get('role');
  const role: AdminUserRole | '' = isUserRole(roleParam) ? roleParam : '';
  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10) || 1);
  const limit = 20;

  const { data, isLoading, error } = useQuery({
    queryKey: [...ADMIN_USERS_QUERY_KEY, { search, role, page }],
    queryFn: () =>
      adminUsersApi.list({
        search: search || undefined,
        role: role || undefined,
        page,
        limit,
      }),
  });

  const updateParams = (updates: Record<string, string | undefined>) => {
    const params = new URLSearchParams(searchParams.toString());
    Object.entries(updates).forEach(([key, value]) => {
      if (value === undefined || value === '') {
        params.delete(key);
      } else {
        params.set(key, value);
      }
    });
    router.push(`${pathname}?${params.toString()}` as any);
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    updateParams({ search: searchInput.trim(), page: '1' });
  };

  const totalPages = data ? Math.ceil(data.total / limit) : 0;

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Пользователи</h1>

      {banner && (
        <div
          role={banner.type === 'error' ? 'alert' : 'status'}
          className={
            banner.type === 'success'
              ? 'rounded-md bg-green-100 p-3 text-sm text-green-900'
              : 'rounded-md bg-destructive/10 p-3 text-sm text-destructive'
          }
        >
          {banner.message}
          <button type="button" onClick={() => setBanner(null)} className="ml-4 underline">
            Скрыть
          </button>
        </div>
      )}

      <form onSubmit={handleSearch} className="flex flex-col gap-2 md:flex-row">
        <Input
          placeholder="Поиск по email или имени"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
        />
        <Button type="submit" variant="outline" aria-label="Найти">
          <Search className="h-4 w-4" />
        </Button>
        <Select
          aria-label="Роль"
          options={ROLE_FILTER_OPTIONS}
          value={role}
          onChange={(e) => updateParams({ role: e.target.value, page: '1' })}
          className="w-full md:w-48"
        />
      </form>

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-destructive">
          Не удалось загрузить пользователей.
        </div>
      )}

      {isLoading ? (
        <div className="h-64 animate-pulse rounded-lg bg-muted" data-testid="users-skeleton" />
      ) : data?.items.length === 0 ? (
        <div className="rounded-lg border p-8 text-center text-muted-foreground">
          Пользователи не найдены.
        </div>
      ) : data ? (
        <>
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Имя</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Роль</TableHead>
                  <TableHead>Email подтверждён</TableHead>
                  <TableHead>Создан</TableHead>
                  <TableHead>Изменить роль</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.items.map((user: AdminUser) => {
                  const isSelf = !!currentUserId && currentUserId === user.id;
                  return (
                    <TableRow key={user.id}>
                      <TableCell className="font-medium">{user.name || '—'}</TableCell>
                      <TableCell>{user.email}</TableCell>
                      <TableCell>{userRoleLabel(user.role)}</TableCell>
                      <TableCell>{user.email_verified_at ? 'Да' : 'Нет'}</TableCell>
                      <TableCell>{formatDate(user.created_at)}</TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <UserRoleSelect
                            user={user}
                            disabled={isSelf}
                            onChanged={(updated, from, to) =>
                              setBanner({
                                type: 'success',
                                message: changeRoleSuccessText(updated, from, to),
                              })
                            }
                            onError={(message) => setBanner({ type: 'error', message })}
                          />
                          {isSelf && (
                            <span className="text-xs text-muted-foreground">Это вы</span>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
          <Pagination
            page={page}
            totalPages={totalPages}
            onPageChange={(p) => updateParams({ page: String(p) })}
          />
        </>
      ) : null}
    </div>
  );
}
