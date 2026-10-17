'use client';

import { loginHref } from '@/shared/lib/safe-redirect';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import {
  type DefaultShelfType,
  defaultTypeLabels,
  defaultTypeOrder,
  isDefaultType,
  useBookShelves,
} from '../model/use-book-shelves';

interface ShelveBookButtonsProps {
  bookId: string;
  slug: string;
  /** System shelves to show buttons for; defaults to all three. */
  types?: DefaultShelfType[];
  /** Whether to show the custom-shelves checklist; defaults to true. */
  showCustomShelves?: boolean;
  /** Guest login redirect target; defaults to `/books/{slug}`. */
  loginRedirect?: string;
}

export function ShelveBookButtons({
  bookId,
  slug,
  types = defaultTypeOrder,
  showCustomShelves = true,
  loginRedirect,
}: ShelveBookButtonsProps) {
  const visibleTypes = defaultTypeOrder.filter((type) => types.includes(type));
  const redirectTarget = loginRedirect ?? `/books/${slug}`;
  const { isAuthenticated, shelves, status, error, toggleDefault, toggleCustom } =
    useBookShelves(bookId);
  const loading = status === 'loading';

  if (!isAuthenticated) {
    return (
      <div className="flex flex-wrap gap-2">
        {visibleTypes.map((type) => (
          <Button key={type} variant="outline" asChild>
            <Link href={loginHref(redirectTarget)}>{defaultTypeLabels[type]}</Link>
          </Button>
        ))}
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex flex-wrap gap-2" aria-busy="true">
        <span className="sr-only">Загрузка полок</span>
        {visibleTypes.map((type) => (
          <div key={type} className="h-10 w-32 animate-pulse rounded-md bg-muted" />
        ))}
      </div>
    );
  }

  const defaultShelves = shelves.filter((s) => isDefaultType(s.type));
  const customShelves = showCustomShelves ? shelves.filter((s) => s.type === 'custom') : [];

  return (
    <div className="space-y-4">
      {error && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <div className="flex flex-wrap gap-2">
        {visibleTypes.map((type) => {
          const shelf = defaultShelves.find((s) => s.type === type);
          if (!shelf) return null;
          return (
            <Button
              key={type}
              variant={shelf.hasBook ? 'default' : 'outline'}
              onClick={() => toggleDefault(shelf)}
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
                  onChange={() => toggleCustom(shelf)}
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
