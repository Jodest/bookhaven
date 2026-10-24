'use client';

import { formatReadCounter } from '@/shared/lib/format-read-count';
import { loginHref } from '@/shared/lib/safe-redirect';
import { Button, Popover, PopoverContent, PopoverTrigger } from '@bookhaven/ui';
import { Bookmark, BookCheck, BookOpen, BookX, Check, ChevronDown, RotateCw, X } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import Link from 'next/link';
import { useState } from 'react';
import {
  defaultTypeLabels,
  isDefaultType,
  statusTypeLabels,
  statusTypeOrder,
  useBookShelves,
} from '../model/use-book-shelves';

const statusIcons: Record<string, LucideIcon> = {
  want_to_read: Bookmark,
  reading: BookOpen,
  read: BookCheck,
  abandoned: BookX,
};

interface BookShelfActionsProps {
  bookId: string;
  slug: string;
}

/**
 * Shelving controls for the book page: one status control (want to read /
 * reading / read / abandoned) with a re-read button, a read counter and a "На полку" popover with the reader's custom shelves.
 */
export function BookShelfActions({ bookId, slug }: BookShelfActionsProps) {
  const { isAuthenticated, shelves, reads, status, error, toggleDefault, toggleCustom, clearStatus } =
    useBookShelves(bookId);
  const [statusOpen, setStatusOpen] = useState(false);

  if (!isAuthenticated) {
    return (
      <Button variant="outline" asChild>
        <Link href={loginHref(`/books/${slug}`)}>{defaultTypeLabels.want_to_read}</Link>
      </Button>
    );
  }

  if (status === 'loading') {
    return (
      <div className="flex flex-wrap gap-2" aria-busy="true">
        <span className="sr-only">Загрузка полок</span>
        <div className="h-10 w-44 animate-pulse rounded-md bg-muted" />
        <div className="h-10 w-28 animate-pulse rounded-md bg-muted" />
      </div>
    );
  }

  const defaultShelves = statusTypeOrder
    .map((type) => shelves.find((s) => s.type === type))
    .filter((s): s is NonNullable<typeof s> => !!s);
  const currentShelf = defaultShelves.find((s) => s.hasBook);
  const wantShelf = defaultShelves.find((s) => s.type === 'want_to_read');
  const customShelves = shelves.filter((s) => s.type === 'custom' && !isDefaultType(s.type));
  const customCount = customShelves.filter((s) => s.hasBook).length;
  const CurrentIcon = currentShelf ? (statusIcons[currentShelf.type] ?? Check) : Check;
  const currentLabel = currentShelf
    ? statusTypeLabels[currentShelf.type as keyof typeof statusTypeLabels]
    : null;
  const readingShelf = defaultShelves.find((s) => s.type === 'reading');
  const rereading = currentShelf?.type === 'reading' && Boolean(reads?.rereading);

  return (
    <div className="space-y-3">
      {error && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>
      )}

      <div className="flex flex-wrap gap-2">
        {defaultShelves.length > 0 && (
          <Popover open={statusOpen} onOpenChange={setStatusOpen}>
            <div className="inline-flex">
              {currentShelf ? (
                <PopoverTrigger asChild>
                  <Button
                    variant="secondary"
                    aria-label={`Статус: ${currentLabel}${rereading ? ', перечитываю' : ''}. Изменить`}
                  >
                    <CurrentIcon className="mr-2 h-4 w-4" aria-hidden="true" />
                    {currentLabel}
                    {rereading && (
                      <span className="ml-2 rounded-full bg-background/60 px-2 py-0.5 text-xs">
                        перечитываю
                      </span>
                    )}
                    <ChevronDown className="ml-2 h-4 w-4" aria-hidden="true" />
                  </Button>
                </PopoverTrigger>
              ) : (
                <>
                  <Button
                    className="rounded-r-none"
                    onClick={() => wantShelf && toggleDefault(wantShelf)}
                    disabled={!wantShelf}
                  >
                    <Bookmark className="mr-2 h-4 w-4" aria-hidden="true" />
                    {defaultTypeLabels.want_to_read}
                  </Button>
                  <PopoverTrigger asChild>
                    <Button
                      className="rounded-l-none border-l border-primary-foreground/30 px-2"
                      aria-label="Выбрать статус"
                    >
                      <ChevronDown className="h-4 w-4" aria-hidden="true" />
                    </Button>
                  </PopoverTrigger>
                </>
              )}
            </div>
            <PopoverContent align="end" className="w-52 space-y-1 p-2">
              {defaultShelves.map((shelf) => {
                const Icon = statusIcons[shelf.type];
                return (
                  <Button
                    key={shelf.id}
                    variant="ghost"
                    className="w-full justify-start"
                    aria-pressed={shelf.hasBook}
                    onClick={() => {
                      setStatusOpen(false);
                      void toggleDefault(shelf);
                    }}
                  >
                    {Icon && <Icon className="mr-2 h-4 w-4" aria-hidden="true" />}
                    {statusTypeLabels[shelf.type as keyof typeof statusTypeLabels]}
                    {shelf.hasBook && <Check className="ml-auto h-4 w-4" aria-hidden="true" />}
                  </Button>
                );
              })}
              {currentShelf && (
                <>
                  <div className="my-1 border-t" role="separator" />
                  <Button
                    variant="ghost"
                    className="w-full justify-start text-muted-foreground"
                    onClick={() => {
                      setStatusOpen(false);
                      void clearStatus();
                    }}
                  >
                    <X className="mr-2 h-4 w-4" aria-hidden="true" />
                    Убрать статус
                  </Button>
                </>
              )}
            </PopoverContent>
          </Popover>
        )}

        {currentShelf?.type === 'read' && readingShelf && (
          <Button variant="secondary" onClick={() => void toggleDefault(readingShelf)}>
            <RotateCw className="mr-2 h-4 w-4" aria-hidden="true" />
            Читаю снова
          </Button>
        )}

        {customShelves.length > 0 && (
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline">
                {customCount > 0 ? `На полку · ${customCount}` : 'На полку'}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-60">
              <p className="mb-2 text-sm font-medium">Добавить на полку</p>
              <div className="space-y-2">
                {customShelves.map((shelf) => (
                  <label key={shelf.id} className="flex cursor-pointer items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={shelf.hasBook || false}
                      onChange={() => toggleCustom(shelf)}
                    />
                    {shelf.name}
                  </label>
                ))}
              </div>
            </PopoverContent>
          </Popover>
        )}
      </div>

      {reads && reads.readsCount >= 1 && (
        <p className="text-sm text-muted-foreground">
          {formatReadCounter(reads.readsCount, reads.lastReadAt)}
        </p>
      )}
    </div>
  );
}
