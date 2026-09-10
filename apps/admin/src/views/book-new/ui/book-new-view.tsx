'use client';

import { adminBooksApi } from '@/entities/book';
import { AdminBookInput } from '@bookhaven/api-client';
import { BookForm } from '@/features/manage-book';

export function BookNewView() {
  const handleCreate = async (data: AdminBookInput) => {
    await adminBooksApi.create(data);
  };

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Новая книга</h1>
      <BookForm onSubmit={handleCreate} />
    </div>
  );
}
