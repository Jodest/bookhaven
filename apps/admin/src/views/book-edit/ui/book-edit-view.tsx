'use client';

import { useParams, useSearchParams } from 'next/navigation';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { adminBooksApi } from '@/entities/book';
import { DASHBOARD_QUERY_KEY } from '@/entities/dashboard';
import { AdminBookInput, ApiError } from '@bookhaven/api-client';
import { BookForm } from '@/features/manage-book';
import { hasWikidataQid } from '@/features/sync-adaptations';
import { safeBackHref, setFlash } from '@/shared/lib';
import { EditPageHeader } from '@/shared/ui';
import { AiDescriptionPanel } from '@/widgets/ai-description-panel';
import { BookAdaptationsAdmin } from '@/widgets/book-adaptations-admin';

// Standalone book edit page (docs/features/catalog-unified-lists.md).
// BookForm navigates to the resolved back href after a save and on «Отмена».

const ALLOWED_BACK = ['/catalog/books', '/moderation/books', '/moderation/descriptions'];
const FALLBACK_BACK = '/catalog/books';
const TITLE = 'Редактирование книги';

export function BookEditView() {
  const params = useParams();
  const id = params.id as string;
  const searchParams = useSearchParams();
  const backHref = safeBackHref(searchParams.get('back'), ALLOWED_BACK, FALLBACK_BACK);
  const queryClient = useQueryClient();

  const { data: book, isLoading, error } = useQuery({
    queryKey: ['admin', 'books', id],
    queryFn: () => adminBooksApi.get(id),
  });

  const handleUpdate = async (data: AdminBookInput) => {
    const updated = await adminBooksApi.update(id, data);
    queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
    queryClient.invalidateQueries({ queryKey: DASHBOARD_QUERY_KEY });
    setFlash('catalog.books', `Книга «${updated.title}» сохранена.`);
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <EditPageHeader backHref={backHref} title={TITLE} />
        <div className="h-64 animate-pulse rounded-lg bg-muted" data-testid="edit-loading" />
      </div>
    );
  }

  if (error || !book) {
    const notFound = error instanceof ApiError && (error.status === 404 || error.status === 400);
    return (
      <div className="space-y-6">
        <EditPageHeader backHref={backHref} title={TITLE} />
        <div className="rounded-md bg-destructive/10 p-6 text-destructive">
          {notFound ? 'Книга не найдена.' : 'Не удалось загрузить данные.'}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <EditPageHeader backHref={backHref} title={TITLE} subtitle={book.title} />
      <BookForm initial={book} onSubmit={handleUpdate} returnHref={backHref} />
      {/* Renders itself only when the book has no own description or has an AI description record. */}
      <AiDescriptionPanel
        bookId={book.id}
        hasOwnDescription={!!book.description?.trim()}
        bookStatus={book.status}
      />
      <BookAdaptationsAdmin bookId={book.id} bookTitle={book.title} hasQid={hasWikidataQid(book)} />
    </div>
  );
}
