'use client';

import { useState } from 'react';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import type { AdminGenreMergeEntry, AdminGenreMergeUndoResult } from '@bookhaven/api-client';
import {
  Button,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@bookhaven/ui';
import { adminGenresApi } from '@/entities/genre';
import { UndoGenreMergeDialog } from '@/features/undo-genre-merge';
import { Pagination } from '@/shared/ui';

// Genre merge history: the merge journal, newest first, with the undo dialog
// (opened by the row button or by ?undo=<merge id>).

const LIMIT = 20;

// «а», «б» → «в»
const mergeTitle = (entry: AdminGenreMergeEntry) =>
  `${entry.sources.map((s) => `«${s.name}»`).join(', ')} → «${entry.target.name}»`;

const formatDate = (iso: string) => new Date(iso).toLocaleDateString('ru-RU');

const undoneMessage = (result: AdminGenreMergeUndoResult) =>
  `Слияние отменено. Восстановлено жанров: ${result.sources_restored}. Вернулось книг: ${result.books_returned}.` +
  (result.collections_switched_back > 0 ? ` Возвращено подборок: ${result.collections_switched_back}.` : '');

export function GenreMergeHistoryView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [banner, setBanner] = useState<string | null>(null);

  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10) || 1);
  const undoId = searchParams.get('undo');

  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'genres', 'merges', { page, limit: LIMIT }],
    queryFn: () => adminGenresApi.merges({ page, limit: LIMIT }),
  });

  const updateParams = (updates: Record<string, string | undefined>) => {
    const next = new URLSearchParams(searchParams.toString());
    Object.entries(updates).forEach(([key, value]) => {
      if (value === undefined || value === '') {
        next.delete(key);
      } else {
        next.set(key, value);
      }
    });
    const query = next.toString();
    router.replace((query ? `${pathname}?${query}` : pathname) as any);
  };

  const items = data?.items ?? [];
  const totalPages = data ? Math.ceil(data.total / LIMIT) : 0;

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h1 className="text-3xl font-bold tracking-tight">История слияний жанров</h1>
        <Link href="/moderation/genres" className="text-sm underline">
          К модерации жанров
        </Link>
      </div>

      {banner && (
        <div className="flex items-center justify-between rounded-md bg-green-100 p-3 text-sm text-green-900">
          {banner}
          <button type="button" onClick={() => setBanner(null)} className="ml-4 underline">
            Скрыть
          </button>
        </div>
      )}

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-destructive">
          Не удалось загрузить историю слияний.
        </div>
      )}

      {isLoading ? (
        <div className="h-64 animate-pulse rounded-lg bg-muted" />
      ) : items.length === 0 ? (
        !error && <div className="rounded-lg border p-8 text-center text-muted-foreground">Слияний пока нет.</div>
      ) : (
        <>
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Когда</TableHead>
                  <TableHead>Кто</TableHead>
                  <TableHead>Слияние</TableHead>
                  <TableHead>Книг перенесено</TableHead>
                  <TableHead>Статус</TableHead>
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((entry) => (
                  <TableRow key={entry.id}>
                    <TableCell className="whitespace-nowrap text-sm">{formatDate(entry.created_at)}</TableCell>
                    <TableCell className="text-sm">{entry.performed_by?.name ?? '—'}</TableCell>
                    <TableCell className="text-sm">{mergeTitle(entry)}</TableCell>
                    <TableCell className="text-sm">{entry.books_reassigned}</TableCell>
                    <TableCell className="whitespace-nowrap text-sm">
                      {entry.undone_at ? `Отменено ${formatDate(entry.undone_at)}` : 'Активно'}
                    </TableCell>
                    <TableCell className="text-right">
                      {!entry.undone_at && (
                        <Button variant="outline" size="sm" onClick={() => updateParams({ undo: entry.id })}>
                          Отменить слияние
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <Pagination
            page={page}
            totalPages={totalPages}
            onPageChange={(p) => updateParams({ page: p > 1 ? String(p) : undefined })}
          />
        </>
      )}

      {undoId && (
        <UndoGenreMergeDialog
          mergeId={undoId}
          onClose={() => updateParams({ undo: undefined })}
          onUndone={(result) => {
            setBanner(undoneMessage(result));
            updateParams({ undo: undefined });
          }}
        />
      )}
    </div>
  );
}
