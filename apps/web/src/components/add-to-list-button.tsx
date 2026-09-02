'use client';

import { api, List } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useAuthStore } from '@/lib/auth-store';

interface AddToListButtonProps {
  bookId: string;
  slug: string;
}

export function AddToListButton({ bookId, slug }: AddToListButtonProps) {
  const { isAuthenticated } = useAuthStore();
  const [lists, setLists] = useState<List[]>([]);
  const [bookInLists, setBookInLists] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [newListTitle, setNewListTitle] = useState('');

  useEffect(() => {
    if (!isAuthenticated) return;

    let cancelled = false;
    setLoading(true);
    api.lists
      .list({ book_id: bookId })
      .then((response) => {
        if (!cancelled) {
          setLists(response.items);
          setBookInLists(new Set(response.book_in_lists ?? []));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Ошибка загрузки списков');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [isAuthenticated, bookId]);

  const handleToggle = async (list: List) => {
    if (!isAuthenticated) return;

    const isInList = bookInLists.has(list.id);

    try {
      if (isInList) {
        await api.lists.removeBook(list.id, bookId);
        setBookInLists((prev) => {
          const next = new Set(prev);
          next.delete(list.id);
          return next;
        });
        setLists((prev) =>
          prev.map((l) => (l.id === list.id ? { ...l, booksCount: Math.max(0, l.booksCount - 1) } : l))
        );
      } else {
        await api.lists.addBook(list.id, bookId);
        setBookInLists((prev) => new Set(prev).add(list.id));
        setLists((prev) =>
          prev.map((l) => (l.id === list.id ? { ...l, booksCount: l.booksCount + 1 } : l))
        );
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка изменения списка');
    }
  };

  const handleCreateList = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newListTitle.trim()) return;

    try {
      const newList = await api.lists.create({
        title: newListTitle.trim(),
        is_public: true,
      });
      setLists((prev) => [newList, ...prev]);
      setNewListTitle('');
      setShowCreate(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ошибка создания списка');
    }
  };

  if (!isAuthenticated) {
    return (
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" asChild>
          <Link href={`/login?redirect=/books/${slug}`}>
            Добавить в список
          </Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {error && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {lists.length > 0 && (
        <div className="rounded-md border p-3">
          <p className="mb-2 text-sm font-medium">Добавить в список</p>
          <div className="space-y-2">
            {lists.map((list) => (
              <label
                key={list.id}
                className="flex cursor-pointer items-center gap-2 text-sm"
              >
                <input
                  type="checkbox"
                  checked={bookInLists.has(list.id)}
                  onChange={() => handleToggle(list)}
                  disabled={loading}
                />
                <span className="flex-1">{list.title}</span>
                <span className="text-xs text-muted-foreground">
                  {list.booksCount}
                </span>
              </label>
            ))}
          </div>
        </div>
      )}

      {showCreate ? (
        <form onSubmit={handleCreateList} className="flex gap-2">
          <input
            value={newListTitle}
            onChange={(e) => setNewListTitle(e.target.value)}
            placeholder="Название нового списка"
            className="flex-1 rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-primary"
          />
          <Button type="submit" size="sm">
            Создать
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setShowCreate(false)}
          >
            Отмена
          </Button>
        </form>
      ) : (
        <Button
          variant="outline"
          size="sm"
          onClick={() => setShowCreate(true)}
        >
          Создать новый список
        </Button>
      )}
    </div>
  );
}
