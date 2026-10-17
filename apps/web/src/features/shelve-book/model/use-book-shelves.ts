'use client';

import { shelvesApi } from '@/entities/shelf';
import { useAuthStore } from '@/entities/session';
import type { Shelf } from '@bookhaven/api-client';
import { useCallback, useEffect, useState } from 'react';

export type DefaultShelfType = 'want_to_read' | 'reading' | 'read';

export const defaultTypeOrder: DefaultShelfType[] = ['want_to_read', 'reading', 'read'];
export const defaultTypeLabels: Record<DefaultShelfType, string> = {
  want_to_read: 'Хочу прочитать',
  reading: 'Читаю',
  read: 'Прочитано',
};

// Shelf.type is a wider union (custom shelves); compare as plain strings.
export const isDefaultType = (type: string) => (defaultTypeOrder as string[]).includes(type);

export type ShelvesStatus = 'loading' | 'ready' | 'error';

/**
 * The reader's shelves for one book, with optimistic toggles. Default
 * (system) shelves are mutually exclusive; custom shelves are independent.
 */
export function useBookShelves(bookId: string) {
  const { isAuthenticated } = useAuthStore();
  const [shelves, setShelves] = useState<Shelf[]>([]);
  const [status, setStatus] = useState<ShelvesStatus>(isAuthenticated ? 'loading' : 'ready');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isAuthenticated) return;

    let cancelled = false;
    setStatus('loading');
    shelvesApi
      .list({ book_id: bookId })
      .then((response) => {
        if (!cancelled) {
          setShelves(response.items);
          setStatus('ready');
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Ошибка загрузки полок');
          setStatus('error');
        }
      });

    return () => {
      cancelled = true;
    };
  }, [isAuthenticated, bookId]);

  const toggleDefault = useCallback(
    async (shelf: Shelf) => {
      if (!isAuthenticated) return;

      const wasActive = shelf.hasBook;
      const targetType = shelf.type;

      setShelves((prev) =>
        prev.map((s) => {
          if (isDefaultType(s.type)) {
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
            if (isDefaultType(s.type)) {
              return { ...s, hasBook: s.type === targetType ? wasActive : s.hasBook };
            }
            return s;
          })
        );
        setError(err instanceof Error ? err.message : 'Ошибка обновления полки');
      }
    },
    [isAuthenticated, bookId]
  );

  const toggleCustom = useCallback(
    async (shelf: Shelf) => {
      if (!isAuthenticated) return;

      const wasActive = shelf.hasBook;

      setShelves((prev) => prev.map((s) => (s.id === shelf.id ? { ...s, hasBook: !wasActive } : s)));

      try {
        if (wasActive) {
          await shelvesApi.removeBook(shelf.id, bookId);
        } else {
          await shelvesApi.addBook(shelf.id, bookId);
        }
      } catch (err) {
        setShelves((prev) => prev.map((s) => (s.id === shelf.id ? { ...s, hasBook: wasActive } : s)));
        setError(err instanceof Error ? err.message : 'Ошибка обновления полки');
      }
    },
    [isAuthenticated, bookId]
  );

  /** Removes the book from whichever default shelf currently holds it. */
  const clearStatus = useCallback(async () => {
    const current = shelves.find((s) => isDefaultType(s.type) && s.hasBook);
    if (current) await toggleDefault(current);
  }, [shelves, toggleDefault]);

  return { isAuthenticated, shelves, status, error, toggleDefault, toggleCustom, clearStatus };
}
