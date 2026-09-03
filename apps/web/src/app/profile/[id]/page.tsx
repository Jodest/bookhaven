'use client';

import { api, PublicProfile } from '@bookhaven/api-client';
import Link from 'next/link';
import { useCallback, useEffect, useRef, useState } from 'react';

interface PublicProfilePageProps {
  params: { id: string };
}

const profileTypeLabels: Record<string, string> = {
  reader: 'Читатель',
  librarian: 'Библиотекарь',
  bookseller: 'Книготорговец',
  other: 'Другое',
};

export default function PublicProfilePage({ params }: PublicProfilePageProps) {
  const [profile, setProfile] = useState<PublicProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const currentIdRef = useRef(params.id);
  currentIdRef.current = params.id;

  const fetchProfile = useCallback(async () => {
    const requestedId = params.id;
    setLoading(true);
    setError(null);
    setProfile(null);
    try {
      const response = await api.users.getPublic(requestedId);
      if (requestedId !== currentIdRef.current) {
        return;
      }
      setProfile(response);
    } catch (err) {
      if (requestedId !== currentIdRef.current) {
        return;
      }
      setError(mapErrorToRussian(err));
    } finally {
      if (requestedId === currentIdRef.current) {
        setLoading(false);
      }
    }
  }, [params.id]);

  useEffect(() => {
    fetchProfile();
  }, [fetchProfile]);

  if (loading) {
    return (
      <main className="container mx-auto px-4 py-8">
        <p className="text-muted-foreground">Загрузка профиля...</p>
      </main>
    );
  }

  if (error || !profile) {
    return (
      <main className="container mx-auto px-4 py-8">
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error || 'Профиль недоступен'}
        </div>
      </main>
    );
  }

  const { user, lists } = profile;
  const displayName = user.name ?? 'Без имени';
  const joinedDate = new Date(user.joinedAt).toLocaleDateString('ru-RU', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  });

  return (
    <main className="container mx-auto px-4 py-8">
      <div className="mx-auto max-w-3xl rounded-lg border bg-card p-8 text-card-foreground shadow-sm">
        <div className="flex flex-col items-center gap-4 sm:flex-row sm:items-start">
          {user.avatarUrl ? (
            <img
              src={user.avatarUrl}
              alt={displayName}
              className="h-24 w-24 rounded-full object-cover"
            />
          ) : (
            <div className="flex h-24 w-24 items-center justify-center rounded-full bg-muted text-2xl text-muted-foreground">
              {displayName[0]}
            </div>
          )}

          <div className="text-center sm:text-left">
            <h1 className="text-3xl font-bold tracking-tight">{displayName}</h1>
            <p className="mt-1 text-muted-foreground">
              {profileTypeLabels[user.profileType] ?? user.profileType}
            </p>
            <p className="text-sm text-muted-foreground">На сайте с {joinedDate}</p>
          </div>
        </div>

        <div className="mt-8">
          <h2 className="text-xl font-semibold">Публичные списки</h2>

          {lists.length === 0 ? (
            <p className="mt-4 text-muted-foreground">Публичных списков пока нет</p>
          ) : (
            <ul className="mt-4 space-y-3">
              {lists.map((list) => (
                <li key={list.id} className="rounded-md border p-4">
                  <Link
                    href={`/lists/public/${list.id}`}
                    className="text-lg font-medium hover:underline"
                  >
                    {list.title}
                  </Link>
                  {list.description && (
                    <p className="mt-1 text-sm text-muted-foreground">{list.description}</p>
                  )}
                  <p className="mt-1 text-sm text-muted-foreground">
                    {list.booksCount} {pluralizeBooks(list.booksCount)}
                  </p>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </main>
  );
}

function pluralizeBooks(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return 'книга';
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return 'книги';
  return 'книг';
}

function mapErrorToRussian(err: unknown): string {
  if (err instanceof Error) {
    const message = err.message.toLowerCase();
    if (message.includes('invalid user id')) {
      return 'Некорректный идентификатор пользователя';
    }
    if (message.includes('user not found')) {
      return 'Профиль не найден или недоступен';
    }
  }
  return 'Ошибка загрузки профиля';
}
