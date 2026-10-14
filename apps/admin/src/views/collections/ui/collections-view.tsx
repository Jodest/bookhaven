'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow, buttonVariants, cn } from '@bookhaven/ui';
import { adminCollectionsApi, COLLECTIONS_QUERY_KEY, KIND_LABELS, shownWarning } from '@/entities/curated-collection';

export function CollectionsView() {
  const { data, isLoading, error } = useQuery({
    queryKey: COLLECTIONS_QUERY_KEY,
    queryFn: () => adminCollectionsApi.list(),
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold tracking-tight">Подборки</h1>
        <Link href={'/collections/new' as any} className={cn(buttonVariants())}>
          Новая подборка
        </Link>
      </div>

      {isLoading && <div className="h-32 animate-pulse rounded-lg bg-muted" data-testid="collections-loading" />}
      {error && (
        <div className="rounded-md bg-destructive/10 p-6 text-destructive">Не удалось загрузить подборки.</div>
      )}
      {data && data.items.length === 0 && <p className="text-muted-foreground">Подборок пока нет.</p>}
      {data && data.items.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Название</TableHead>
              <TableHead>Тип</TableHead>
              <TableHead>Статус</TableHead>
              <TableHead>Показано</TableHead>
              <TableHead>Книг</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.items.map((c) => {
              const warning = shownWarning(c.kind, c.shown_count, c.size);
              return (
                <TableRow key={c.id}>
                  <TableCell className="font-medium">{c.title}</TableCell>
                  <TableCell>{KIND_LABELS[c.kind]}</TableCell>
                  <TableCell>{c.is_published ? 'Опубликована' : 'Черновик'}</TableCell>
                  <TableCell>
                    {c.shown_count} из {c.size}
                    {warning && (
                      <span
                        role="img"
                        aria-label={warning}
                        title={warning}
                        data-testid="reserve-warning"
                        className="ml-2 text-amber-600"
                      >
                        ⚠
                      </span>
                    )}
                  </TableCell>
                  <TableCell>{c.books_count}</TableCell>
                  <TableCell>
                    <Link href={`/collections/${c.id}` as any} className="text-primary hover:underline">
                      Редактировать
                    </Link>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
