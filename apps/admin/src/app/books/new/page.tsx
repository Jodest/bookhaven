'use client';

import { api, AdminBookInput } from '@bookhaven/api-client';
import { BookForm } from '@/components/book-form';

export default function NewBookPage() {
  const handleCreate = async (data: AdminBookInput) => {
    await api.admin.books.create(data);
  };

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Новая книга</h1>
      <BookForm onSubmit={handleCreate} />
    </div>
  );
}
