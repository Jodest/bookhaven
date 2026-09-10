'use client';

import { useParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { adminBooksApi } from '@/entities/book';
import { AdminBookInput } from '@bookhaven/api-client';
import { BookForm } from '@/features/manage-book';

export function BookEditView() {
  const params = useParams();
  const id = params.id as string;

  const { data: book, isLoading, error } = useQuery({
    queryKey: ['admin', 'books', id],
    queryFn: () => adminBooksApi.get(id),
  });

  const handleUpdate = async (data: AdminBookInput) => {
    await adminBooksApi.update(id, data);
  };

  if (isLoading) {
    return <div className="h-64 animate-pulse rounded-lg bg-muted" />;
  }

  if (error || !book) {
    return (
      <div className="rounded-md bg-destructive/10 p-6 text-destructive">
        Книга не найдена или не удалось загрузить данные.
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Редактировать книгу</h1>
      <BookForm initial={book} onSubmit={handleUpdate} />
    </div>
  );
}
