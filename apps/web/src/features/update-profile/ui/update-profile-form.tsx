'use client';

import { useAuthStore } from '@/entities/session';
import { api, UpdateProfileInput } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';

const profileTypeOptions = [
  { value: 'reader', label: 'Читатель' },
  { value: 'librarian', label: 'Библиотекарь' },
  { value: 'bookseller', label: 'Книготорговец' },
  { value: 'other', label: 'Другое' },
] as const;

function mapErrorToRussian(err: unknown): string {
  if (err instanceof Error) {
    const message = err.message.toLowerCase();
    if (message.includes('name')) {
      return 'Имя должно содержать от 2 до 255 символов';
    }
    if (message.includes('avatar')) {
      return 'Ссылка на аватар должна быть корректным URL';
    }
    if (message.includes('profile_type')) {
      return 'Выберите корректный тип профиля';
    }
  }
  return 'Не удалось сохранить профиль. Попробуйте позже.';
}

export function UpdateProfileForm() {
  const { user, logout, setUser } = useAuthStore();
  const router = useRouter();

  const [name, setName] = useState('');
  const [avatarUrl, setAvatarUrl] = useState('');
  const [profileType, setProfileType] = useState<string>('reader');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  useEffect(() => {
    if (user) {
      setName(user.name ?? '');
      setAvatarUrl(user.avatarUrl ?? '');
      setProfileType(user.profileType);
      setError(null);
      setSuccess(false);
    }
  }, [user]);

  const handleLogout = async () => {
    await logout();
    router.push('/');
  };

  const validate = (): string | null => {
    const trimmed = name.trim();
    const length = Array.from(trimmed).length;
    if (length < 2 || length > 255) {
      return 'Имя должно содержать от 2 до 255 символов';
    }
    if (avatarUrl.trim()) {
      try {
        const url = new URL(avatarUrl.trim());
        if (url.protocol !== 'http:' && url.protocol !== 'https:') {
          return 'Ссылка на аватар должна начинаться с http:// или https://';
        }
      } catch {
        return 'Введите корректную ссылку на аватар';
      }
    }
    if (!profileTypeOptions.some((o) => o.value === profileType)) {
      return 'Выберите корректный тип профиля';
    }
    return null;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSuccess(false);

    const validationError = validate();
    if (validationError) {
      setError(validationError);
      return;
    }

    setSaving(true);
    try {
      const data: UpdateProfileInput = {
        name: name.trim(),
        avatarUrl: avatarUrl.trim() || null,
        profileType: profileType as UpdateProfileInput['profileType'],
      };
      const updated = await api.users.updateMe(data);
      setUser(updated);
      setSuccess(true);
    } catch (err) {
      setError(mapErrorToRussian(err));
    } finally {
      setSaving(false);
    }
  };

  if (!user) {
    return null;
  }

  return (
    <form onSubmit={handleSubmit} className="mt-6 space-y-5">
      <div>
        <label htmlFor="name" className="block text-sm font-medium text-muted-foreground">
          Имя
        </label>
        <input
          id="name"
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={saving}
          className="mt-1 block w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-50"
        />
      </div>

      <div>
        <label htmlFor="avatar" className="block text-sm font-medium text-muted-foreground">
          Ссылка на аватар
        </label>
        <input
          id="avatar"
          type="text"
          value={avatarUrl}
          onChange={(e) => setAvatarUrl(e.target.value)}
          disabled={saving}
          placeholder="https://example.com/avatar.jpg"
          className="mt-1 block w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-50"
        />
        {avatarUrl.trim() ? (
          <img
            src={avatarUrl.trim()}
            alt="Аватар"
            className="mt-3 h-24 w-24 rounded-full object-cover"
            onError={(e) => {
              (e.target as HTMLImageElement).style.display = 'none';
            }}
          />
        ) : (
          <div className="mt-3 flex h-24 w-24 items-center justify-center rounded-full bg-muted text-muted-foreground">
            {user.name?.[0] ?? user.email[0]}
          </div>
        )}
      </div>

      <div>
        <label htmlFor="profileType" className="block text-sm font-medium text-muted-foreground">
          Тип профиля
        </label>
        <select
          id="profileType"
          value={profileType}
          onChange={(e) => setProfileType(e.target.value)}
          disabled={saving}
          className="mt-1 block w-full rounded-md border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-50"
        >
          {profileTypeOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </div>

      <div>
        <dt className="text-sm text-muted-foreground">Email</dt>
        <dd className="text-lg font-medium">{user.email}</dd>
      </div>

      {error && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {success && (
        <div className="rounded-md bg-green-100 p-3 text-sm text-green-800">
          Профиль сохранён
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3 pt-2">
        <Button type="submit" disabled={saving}>
          {saving ? 'Сохранение...' : 'Сохранить'}
        </Button>
        <Button type="button" variant="outline" onClick={handleLogout} disabled={saving}>
          Выйти
        </Button>
      </div>
    </form>
  );
}
