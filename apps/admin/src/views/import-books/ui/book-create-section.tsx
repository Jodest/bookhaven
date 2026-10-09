'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useQueryClient } from '@tanstack/react-query';
import { AdminBookInput } from '@bookhaven/api-client';
import { adminBooksApi } from '@/entities/book';
import { BookForm } from '@/features/manage-book';

interface CreatedBook {
  id: string;
  title: string;
}

/**
 * Manual book creation on /import/books (docs/features/import-by-entity.md):
 * the form stays on the page, resets after a successful create and a banner
 * links to the created book.
 */
export function BookCreateSection() {
  const queryClient = useQueryClient();
  const [created, setCreated] = useState<CreatedBook | null>(null);

  const handleCreate = async (data: AdminBookInput) => {
    setCreated(null);
    const book = await adminBooksApi.create(data);
    queryClient.invalidateQueries({ queryKey: ['admin', 'books'] });
    setCreated({ id: book.id, title: book.title });
  };

  return (
    <div className="space-y-4">
      {created && (
        <div
          role="status"
          className="flex max-w-2xl items-start justify-between gap-4 rounded-md bg-green-100 p-3 text-sm text-green-900"
        >
          <div className="space-y-1">
            <div>Книга «{created.title}» создана</div>
            <Link href={`/catalog/books/${created.id}/edit` as any} className="underline">
              Открыть
            </Link>
          </div>
          <button type="button" onClick={() => setCreated(null)} className="underline">
            Скрыть
          </button>
        </div>
      )}
      <BookForm onSubmit={handleCreate} stayOnSuccess />
    </div>
  );
}
