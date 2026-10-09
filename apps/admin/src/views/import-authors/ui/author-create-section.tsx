'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { AdminAuthorInput } from '@bookhaven/api-client';
import { adminAuthorsApi } from '@/entities/author';
import { AuthorForm } from '@/features/manage-author';

/**
 * Manual author creation on /import/authors (docs/features/import-by-entity.md),
 * the create half lifted from the catalog AuthorsView. Authors are created
 * published by the server; the form hides the status field when creating.
 */
export function AuthorCreateSection() {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState<AdminAuthorInput>({ name: '' });
  const [createdName, setCreatedName] = useState<string | null>(null);

  const createMutation = useMutation({
    mutationFn: adminAuthorsApi.create,
    onSuccess: (author) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
      setFormData({ name: '' });
      setCreatedName(author.name);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setCreatedName(null);
    createMutation.mutate(formData);
  };

  return (
    <div className="space-y-4">
      {createdName !== null && (
        <div
          role="status"
          className="flex items-start justify-between gap-4 rounded-md bg-green-100 p-3 text-sm text-green-900"
        >
          <div className="space-y-1">
            <div>Автор создан</div>
            <Link
              href={`/catalog/authors?search=${encodeURIComponent(createdName)}` as any}
              className="underline"
            >
              Открыть в каталоге
            </Link>
          </div>
          <button type="button" onClick={() => setCreatedName(null)} className="underline">
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
        successMessage={null}
        onDismissSuccess={() => setCreatedName(null)}
        errorMessage={createMutation.error?.message}
      />
    </div>
  );
}
