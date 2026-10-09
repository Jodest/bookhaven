'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { AdminAuthorInput } from '@bookhaven/api-client';
import { adminAuthorsApi } from '@/entities/author';
import { AuthorForm } from '@/features/manage-author';

/**
 * Manual author creation on /import/authors/manual (docs/features/import-by-entity.md),
 * the create half lifted from the catalog AuthorsView. Authors are created
 * published by the server; the form hides the status field when creating.
 */
export function AuthorCreateSection() {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState<AdminAuthorInput>({ name: '' });
  const [created, setCreated] = useState<{ id: string; name: string } | null>(null);

  const createMutation = useMutation({
    mutationFn: adminAuthorsApi.create,
    onSuccess: (author) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
      setFormData({ name: '' });
      setCreated({ id: author.id, name: author.name });
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setCreated(null);
    createMutation.mutate(formData);
  };

  return (
    <div className="space-y-4">
      {created !== null && (
        <div
          role="status"
          className="flex items-start justify-between gap-4 rounded-md bg-green-100 p-3 text-sm text-green-900"
        >
          <div className="space-y-1">
            <div>Автор создан</div>
            <Link
              href={`/catalog/authors/${encodeURIComponent(created.id)}/edit` as any}
              className="underline"
            >
              Открыть в каталоге
            </Link>
          </div>
          <button type="button" onClick={() => setCreated(null)} className="underline">
            Скрыть
          </button>
        </div>
      )}
      <AuthorForm
        formData={formData}
        onFieldChange={(patch) => setFormData((prev) => ({ ...prev, ...patch }))}
        editing={null}
        onSubmit={handleSubmit}
        onCancel={() => setFormData({ name: '' })}
        pending={createMutation.isPending}
        errorMessage={createMutation.error?.message}
      />
    </div>
  );
}
