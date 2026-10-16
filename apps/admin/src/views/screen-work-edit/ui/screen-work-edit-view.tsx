'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useParams, useRouter, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { ApiError } from '@bookhaven/api-client';
import { Badge, Button } from '@bookhaven/ui';
import { adminScreenWorksApi, deleteWorkErrorText } from '@/entities/screen-work';
import { ScreenWorkEditForm, useDeleteScreenWork } from '@/features/edit-screen-work';
import { safeBackHref, setFlash } from '@/shared/lib';
import { EditPageHeader } from '@/shared/ui';

// «Редактирование фильма» (openspec screen-works/admin-editing): the work
// form, its linked books and authors, and deletion of an unlinked work.
// `back` may point to the work list or to a book / author edit page.

const LIST_PATH = '/catalog/screen-works';
const FLASH_KEY = 'catalog.screen-works';
const TITLE = 'Редактирование фильма';
const OWNER_EDIT_PATH = /^\/catalog\/(?:books|authors)\/[A-Za-z0-9-]+\/edit$/;

/**
 * Same-origin back URL: the work list, or one book / author edit page. The
 * pathname is matched exactly by safeBackHref, which also rejects anything
 * that is not a plain relative path.
 */
export function resolveBackHref(raw: string | null): string {
  const path = (raw ?? '').split('?')[0];
  const allowed = [LIST_PATH];
  if (OWNER_EDIT_PATH.test(path)) allowed.push(path);
  return safeBackHref(raw, allowed, LIST_PATH);
}

export function ScreenWorkEditView() {
  const params = useParams();
  const id = params.id as string;
  const router = useRouter();
  const searchParams = useSearchParams();
  const backHref = resolveBackHref(searchParams.get('back'));
  const backIsList = backHref.split('?')[0] === LIST_PATH;
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const deleteMutation = useDeleteScreenWork(id);

  const { data: work, isLoading, error } = useQuery({
    queryKey: ['admin', 'screen-works', id],
    queryFn: () => adminScreenWorksApi.get(id),
  });

  if (isLoading) {
    return (
      <div className="space-y-6">
        <EditPageHeader backHref={backHref} title={TITLE} />
        <div className="h-64 animate-pulse rounded-lg bg-muted" data-testid="edit-loading" />
      </div>
    );
  }

  if (error || !work) {
    const notFound = error instanceof ApiError && (error.status === 404 || error.status === 400);
    return (
      <div className="space-y-6">
        <EditPageHeader backHref={backHref} title={TITLE} />
        <div className="rounded-md bg-destructive/10 p-6 text-destructive">
          {notFound ? 'Фильм не найден.' : 'Не удалось загрузить данные.'}
        </div>
      </div>
    );
  }

  const linked = work.books.length + work.authors.length > 0;

  const leave = (message: string) => {
    if (backIsList) setFlash(FLASH_KEY, message);
    router.push(backHref as any);
  };

  const handleDelete = () => {
    setDeleteError(null);
    if (!confirm(`Удалить фильм «${work.title}» из каталога?`)) return;
    deleteMutation.mutate(undefined, {
      onSuccess: () => leave(`Фильм «${work.title}» удалён.`),
      onError: (err) => setDeleteError(deleteWorkErrorText(err)),
    });
  };

  return (
    <div className="space-y-6">
      <EditPageHeader backHref={backHref} title={TITLE} subtitle={work.title} />
      {work.manually_edited_at && (
        <p className="text-sm text-muted-foreground">
          Изменён вручную {new Date(work.manually_edited_at).toLocaleDateString('ru-RU')}
        </p>
      )}

      <ScreenWorkEditForm
        key={work.manually_edited_at ?? 'original'}
        work={work}
        onSaved={(updated) => leave(`Фильм «${updated.title}» сохранён.`)}
        onCancel={() => router.push(backHref as any)}
      />

      <section className="space-y-3 rounded-lg border p-4">
        <h2 className="text-lg font-semibold">Связи</h2>
        {!linked && <p className="text-sm text-muted-foreground">Связей с книгами и авторами нет.</p>}
        {work.books.length > 0 && (
          <div className="space-y-1">
            <h3 className="text-sm font-medium">Книги</h3>
            <ul className="space-y-1">
              {work.books.map((book) => (
                <li key={book.id} className="flex flex-wrap items-center gap-2">
                  <Link href={`/catalog/books/${encodeURIComponent(book.id)}/edit` as any} className="text-primary hover:underline">
                    {book.title}
                  </Link>
                  {book.hidden && <Badge variant="outline">Скрыта</Badge>}
                  {book.source === 'manual' && <Badge variant="secondary">Вручную</Badge>}
                </li>
              ))}
            </ul>
          </div>
        )}
        {work.authors.length > 0 && (
          <div className="space-y-1">
            <h3 className="text-sm font-medium">Авторы</h3>
            <ul className="space-y-1">
              {work.authors.map((author) => (
                <li key={author.id} className="flex flex-wrap items-center gap-2">
                  <Link
                    href={`/catalog/authors/${encodeURIComponent(author.id)}/edit` as any}
                    className="text-primary hover:underline"
                  >
                    {author.name}
                  </Link>
                  {author.relation === 'documentary' && <Badge variant="secondary">Документальный</Badge>}
                  {author.hidden && <Badge variant="outline">Скрыт</Badge>}
                  {author.source === 'manual' && <Badge variant="secondary">Вручную</Badge>}
                </li>
              ))}
            </ul>
          </div>
        )}
      </section>

      <section className="space-y-2">
        {linked ? (
          <p className="text-sm text-muted-foreground">
            Фильм можно удалить, только когда у него нет связей с книгами и авторами.
          </p>
        ) : (
          <Button
            type="button"
            variant="outline"
            className="text-destructive"
            disabled={deleteMutation.isPending}
            onClick={handleDelete}
          >
            Удалить фильм
          </Button>
        )}
        {deleteError && (
          <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            {deleteError}
          </div>
        )}
      </section>
    </div>
  );
}
