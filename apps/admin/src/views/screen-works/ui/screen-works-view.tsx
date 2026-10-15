'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { Button, Input, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@bookhaven/ui';
import { Search } from 'lucide-react';
import { adminScreenWorksApi, kindLabel } from '@/entities/screen-work';
import { buildEditHref, consumeFlash } from '@/shared/lib';
import { Pagination, WikidataLink } from '@/shared/ui';

// «Фильмы и сериалы» (openspec screen-works/admin-editing): all screen
// works with search, the «unlinked» filter and paging kept in the address.
// Works have no status, so this is a plain list, not widgets/catalog-list.
// There is no create action: works are created from a book or an author.

const LIMIT = 20;
const FLASH_KEY = 'catalog.screen-works';

export function ScreenWorksView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const search = searchParams.get('search') ?? '';
  const unlinked = searchParams.get('unlinked') === 'true';
  const pageParam = Number(searchParams.get('page'));
  const page = Number.isInteger(pageParam) && pageParam >= 1 ? pageParam : 1;

  const [searchInput, setSearchInput] = useState(search);
  const [flash, setFlash] = useState<string | null>(null);

  useEffect(() => {
    setSearchInput(search);
  }, [search]);

  useEffect(() => {
    setFlash(consumeFlash(FLASH_KEY));
  }, []);

  const pushParams = (patch: Record<string, string | undefined>) => {
    const next = new URLSearchParams(searchParams.toString());
    for (const [key, value] of Object.entries(patch)) {
      if (value === undefined || value === '') next.delete(key);
      else next.set(key, value);
    }
    const qs = next.toString();
    router.push((qs ? `${pathname}?${qs}` : pathname) as any);
  };

  const params = { search: search || undefined, unlinked: unlinked || undefined, page, limit: LIMIT };
  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'screen-works', params],
    queryFn: () => adminScreenWorksApi.list(params),
  });

  const items = data?.items ?? [];
  const totalPages = data ? Math.ceil(data.total / LIMIT) : 0;

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Фильмы и сериалы</h1>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          pushParams({ search: searchInput.trim(), page: undefined });
        }}
        className="flex flex-col gap-2 md:flex-row md:items-center"
      >
        <Input
          placeholder="Поиск по названию, QID, IMDb или Кинопоиску"
          aria-label="Поиск"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
        />
        <Button type="submit" variant="outline" aria-label="Найти">
          <Search className="h-4 w-4" />
        </Button>
        <label className="flex items-center gap-2 whitespace-nowrap text-sm">
          <input
            type="checkbox"
            checked={unlinked}
            onChange={(e) => pushParams({ unlinked: e.target.checked ? 'true' : undefined, page: undefined })}
          />
          Только без связей
        </label>
      </form>

      {flash && (
        <div className="rounded-md bg-green-100 p-3 text-sm text-green-900" role="status">
          {flash}
        </div>
      )}

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-destructive">Не удалось загрузить фильмы.</div>
      )}

      {isLoading ? (
        <div className="h-64 animate-pulse rounded-lg bg-muted" data-testid="screen-works-loading" />
      ) : items.length === 0 ? (
        !error && <div className="rounded-lg border p-8 text-center text-muted-foreground">Фильмы не найдены.</div>
      ) : (
        <>
          <div className="rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Название</TableHead>
                  <TableHead>Год</TableHead>
                  <TableHead>Тип</TableHead>
                  <TableHead>Режиссёр</TableHead>
                  <TableHead>QID</TableHead>
                  <TableHead>Связи</TableHead>
                  <TableHead className="text-right">Действия</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((work) => (
                  <TableRow key={work.id}>
                    <TableCell className="font-medium">{work.title}</TableCell>
                    <TableCell>{work.year ?? ''}</TableCell>
                    <TableCell>{kindLabel(work.kind)}</TableCell>
                    <TableCell>{work.director ?? ''}</TableCell>
                    <TableCell>
                      <WikidataLink qid={work.qid} />
                    </TableCell>
                    <TableCell className="whitespace-nowrap text-sm">
                      <div>Книг: {work.book_count}</div>
                      <div>Авторов: {work.author_count}</div>
                    </TableCell>
                    <TableCell className="text-right">
                      <Button asChild variant="ghost" size="sm">
                        <Link
                          href={
                            buildEditHref(
                              `/catalog/screen-works/${encodeURIComponent(work.id)}/edit`,
                              pathname,
                              searchParams
                            ) as any
                          }
                        >
                          Изменить
                        </Link>
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <Pagination page={page} totalPages={totalPages} onPageChange={(p) => pushParams({ page: String(p) })} />
        </>
      )}
    </div>
  );
}
