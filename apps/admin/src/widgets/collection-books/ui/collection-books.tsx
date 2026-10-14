'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { AdminCuratedCollectionDetail, AdminCuratedCollectionItem } from '@bookhaven/api-client';
import { Button, Input, Label } from '@bookhaven/ui';
import { adminBooksApi, BookStatusBadge } from '@/entities/book';
import { ITEM_STATE_LABELS } from '@/entities/curated-collection';
import { bookErrorText, useCollectionBookMutations } from '@/features/edit-collection-books';
import { PasteBooksPanel } from '@/features/paste-collection-books';

const SEARCH_LIMIT = 10;

interface CollectionBooksProps {
  collection: AdminCuratedCollectionDetail;
}

function authorsText(authors: { name: string }[]): string {
  return authors.map((a) => a.name).join(', ');
}

interface RowProps {
  item: AdminCuratedCollectionItem;
  isFirst: boolean;
  isLast: boolean;
  pending: boolean;
  onMove: (bookId: string, position: number) => void;
  onRemove: (bookId: string) => void;
}

function BookRow({ item, isFirst, isLast, pending, onMove, onRemove }: RowProps) {
  const [target, setTarget] = useState('');
  const { book } = item;

  const submitMove = (e: React.FormEvent) => {
    e.preventDefault();
    const position = Number(target);
    if (!Number.isInteger(position) || position < 1) return;
    onMove(book.id, position);
    setTarget('');
  };

  return (
    <tr className="border-b" data-state={item.state}>
      <td className="w-40 p-2 align-top">
        {item.state === 'ranked' ? (
          <span className="font-semibold">№ {item.place}</span>
        ) : (
          <span className="text-sm text-muted-foreground">{ITEM_STATE_LABELS[item.state]}</span>
        )}
        <div className="text-xs text-muted-foreground">позиция {item.position}</div>
      </td>
      <td className="p-2 align-top">
        <div className="font-medium">{book.title}</div>
        <div className="text-sm text-muted-foreground">{authorsText(book.authors)}</div>
        <div className="mt-1">
          <BookStatusBadge status={book.status} />
        </div>
      </td>
      <td className="p-2 align-top">
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={pending || isFirst}
            onClick={() => onMove(book.id, item.position - 1)}
          >
            Выше
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={pending || isLast}
            onClick={() => onMove(book.id, item.position + 1)}
          >
            Ниже
          </Button>
          <form onSubmit={submitMove} className="flex items-center gap-1">
            <Input
              type="number"
              min={1}
              className="h-8 w-20"
              aria-label="Переместить на"
              placeholder="Переместить на"
              value={target}
              onChange={(e) => setTarget(e.target.value)}
            />
            <Button type="submit" size="sm" variant="outline" disabled={pending || target === ''}>
              Переместить
            </Button>
          </form>
          <Button type="button" size="sm" variant="outline" disabled={pending} onClick={() => onRemove(book.id)}>
            Убрать
          </Button>
        </div>
      </td>
    </tr>
  );
}

/**
 * Ordered book list of a manual collection (places, reserve divider, hidden
 * markers, reorder controls) plus the catalog search that adds books.
 */
export function CollectionBooks({ collection }: CollectionBooksProps) {
  const { add, remove, move } = useCollectionBookMutations(collection.id);
  const [draft, setDraft] = useState('');
  const [term, setTerm] = useState('');
  const [pasteOpen, setPasteOpen] = useState(false);

  const search = useQuery({
    queryKey: ['admin', 'books', 'collection-search', term],
    queryFn: () => adminBooksApi.list({ search: term, limit: SEARCH_LIMIT }),
    enabled: term.length > 0,
  });

  const present = new Set(collection.items.map((i) => i.book.id));
  const pending = add.isPending || remove.isPending || move.isPending;
  const error = bookErrorText(add.error ?? remove.error ?? move.error);
  const firstReserve = collection.items.findIndex((i) => i.state === 'reserve');

  const submitSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setTerm(draft.trim());
  };

  return (
    <div className="space-y-6">
      <section className="space-y-3 rounded-lg border p-4">
        <form onSubmit={submitSearch} className="flex items-end gap-2">
          <div className="flex-1 space-y-2">
            <Label htmlFor="collection-book-search">Найти книгу в каталоге</Label>
            <Input
              id="collection-book-search"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              placeholder="Название книги"
            />
          </div>
          <Button type="submit" variant="outline">
            Найти
          </Button>
        </form>
        {search.isLoading && <p className="text-sm text-muted-foreground">Поиск…</p>}
        {search.data && search.data.items.length === 0 && (
          <p className="text-sm text-muted-foreground">Ничего не найдено.</p>
        )}
        {search.data && search.data.items.length > 0 && (
          <ul className="divide-y">
            {search.data.items.map((b) => {
              const inCollection = present.has(b.id);
              const disabled = pending || inCollection || b.status === 'rejected';
              return (
                <li key={b.id} className="flex items-center justify-between gap-3 py-2">
                  <div>
                    <div className="font-medium">{b.title}</div>
                    <div className="text-sm text-muted-foreground">{authorsText(b.authors)}</div>
                  </div>
                  <div className="flex items-center gap-2">
                    <BookStatusBadge status={b.status} />
                    {inCollection && <span className="text-xs text-muted-foreground">Уже в подборке</span>}
                    <Button
                      type="button"
                      size="sm"
                      disabled={disabled}
                      onClick={() => add.mutate({ bookId: b.id })}
                    >
                      Добавить
                    </Button>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>

      <div className="space-y-3">
        <Button
          type="button"
          variant="outline"
          aria-expanded={pasteOpen}
          onClick={() => setPasteOpen((open) => !open)}
        >
          Вставить список
        </Button>
        {pasteOpen && <PasteBooksPanel collectionId={collection.id} />}
      </div>

      {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}

      {collection.items.length === 0 ? (
        <p className="text-sm text-muted-foreground">В подборке пока нет книг.</p>
      ) : (
        <table className="w-full text-left text-sm">
          <tbody>
            {collection.items.map((item, idx) => (
              <FragmentRow
                key={item.book.id}
                showDivider={idx === firstReserve}
                item={item}
                isFirst={idx === 0}
                isLast={idx === collection.items.length - 1}
                pending={pending}
                onMove={(bookId, position) => move.mutate({ bookId, position })}
                onRemove={(bookId) => remove.mutate(bookId)}
              />
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function FragmentRow({ showDivider, ...row }: RowProps & { showDivider: boolean }) {
  return (
    <>
      {showDivider && (
        <tr data-testid="reserve-divider" className="bg-muted">
          <td colSpan={3} className="p-2 text-xs font-medium uppercase text-muted-foreground">
            Ниже — резерв, читатели его не видят
          </td>
        </tr>
      )}
      <BookRow {...row} />
    </>
  );
}
