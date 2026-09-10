'use client';

import { shelvesApi } from '@/entities/shelf';
import { useAuthStore } from '@/entities/session';
import { Shelf } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useEffect, useState } from 'react';

interface ShelveBookButtonsProps {
  bookId: string;
  slug: string;
}

const defaultTypeLabels: Record<string, string> = {
  want_to_read: 'Хочу прочитать',
  reading: 'Читаю',
  read: 'Прочитано',
};

const defaultTypeOrder = ['want_to_read', 'reading', 'read'];

export function ShelveBookButtons({ bookId, slug }: ShelveBookButtonsProps) {
  const { isAuthenticated } = useAuthStore();
  const [shelves, setShelves] = useState<Shelf[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isAuthenticated) return;

    let cancelled = false;
    setLoading(true);
    shelvesApi
      .list({ book_id: bookId })
      .then((response) => {
        if (!cancelled) {
          setShelves(response.items);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Ошибка загрузки полок');
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

  const handleDefaultToggle = async (shelf: Shelf) => {
    if (!isAuthenticated) return;

    const wasActive = shelf.hasBook;
    const targetType = shelf.type;

    setShelves((prev) =>
      prev.map((s) => {
        if (defaultTypeOrder.includes(s.type)) {
          return { ...s, hasBook: s.type === targetType ? !wasActive : false };
        }
        return s;
      })
    );

    try {
      if (wasActive) {
        await shelvesApi.removeBook(shelf.id, bookId);
      } else {
        await shelvesApi.addBook(shelf.id, bookId);
      }
    } catch (err) {
      setShelves((prev) =>
        prev.map((s) => {
          if (defaultTypeOrder.includes(s.type)) {
            return { ...s, hasBook: s.type === targetType ? wasActive : s.hasBook };
          }
          return s;
        })
      );
      setError(err instanceof Error ? err.message : 'Ошибка обновления полки');
    }
  };

  const handleCustomToggle = async (shelf: Shelf) => {
    if (!isAuthenticated) return;

    const wasActive = shelf.hasBook;

    setShelves((prev) =>
      prev.map((s) => (s.id === shelf.id ? { ...s, hasBook: !wasActive } : s))
    );

    try {
      if (wasActive) {
        await shelvesApi.removeBook(shelf.id, bookId);
      } else {
        await shelvesApi.addBook(shelf.id, bookId);
      }
    } catch (err) {
      setShelves((prev) =>
        prev.map((s) => (s.id === shelf.id ? { ...s, hasBook: wasActive } : s))
      );
      setError(err instanceof Error ? err.message : 'Ошибка обновления полки');
    }
  };

  if (!isAuthenticated) {
    return (
      <div className="flex flex-wrap gap-2">
        {defaultTypeOrder.map((type) => (
          <Button key={type} variant="outline" asChild>
            <Link href={`/login?redirect=/books/${slug}`}>
              {defaultTypeLabels[type]}
            </Link>
          </Button>
        ))}
      </div>
    );
  }

  const defaultShelves = shelves.filter((s) => defaultTypeOrder.includes(s.type));
  const customShelves = shelves.filter((s) => s.type === 'custom');

  return (
    <div className="space-y-4">
      {error && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <div className="flex flex-wrap gap-2">
        {defaultTypeOrder.map((type) => {
          const shelf = defaultShelves.find((s) => s.type === type);
          if (!shelf) return null;
          return (
            <Button
              key={type}
              variant={shelf.hasBook ? 'default' : 'outline'}
              onClick={() => handleDefaultToggle(shelf)}
              disabled={loading}
            >
              {defaultTypeLabels[type]}
            </Button>
          );
        })}
      </div>

      {customShelves.length > 0 && (
        <div className="rounded-md border p-3">
          <p className="mb-2 text-sm font-medium">Добавить на полку</p>
          <div className="space-y-2">
            {customShelves.map((shelf) => (
              <label
                key={shelf.id}
                className="flex cursor-pointer items-center gap-2 text-sm"
              >
                <input
                  type="checkbox"
                  checked={shelf.hasBook || false}
                  onChange={() => handleCustomToggle(shelf)}
                  disabled={loading}
                />
                {shelf.name}
              </label>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
