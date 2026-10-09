'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { AdminGenreInput } from '@bookhaven/api-client';
import { adminGenresApi } from '@/entities/genre';
import { GenreForm, genreMutationErrorText } from '@/features/manage-genre';

/**
 * Manual genre creation on /import/genres (docs/features/import-by-entity.md),
 * the create half lifted from the catalog GenresView. Manually created genres
 * are published right away by the server.
 */
export function GenreCreateSection() {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState<AdminGenreInput>({ name: '' });
  const [createdName, setCreatedName] = useState<string | null>(null);

  const createMutation = useMutation({
    mutationFn: adminGenresApi.create,
    onSuccess: (genre) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'genres'] });
      setFormData({ name: '' });
      setCreatedName(genre.name);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setCreatedName(null);
    createMutation.mutate(formData);
  };

  return (
    <div className="space-y-4">
      <GenreForm
        formData={formData}
        onFieldChange={(patch) => setFormData((prev) => ({ ...prev, ...patch }))}
        editing={null}
        onSubmit={handleSubmit}
        onCancel={() => setFormData({ name: '' })}
        pending={createMutation.isPending}
      />
      {createdName !== null && (
        <div
          role="status"
          className="flex items-start justify-between gap-4 rounded-md bg-green-100 p-3 text-sm text-green-900"
        >
          <div className="space-y-1">
            <div>Жанр создан</div>
            <Link
              href={`/catalog/genres?search=${encodeURIComponent(createdName)}` as any}
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
      {createMutation.error && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {genreMutationErrorText(createMutation.error)}
        </div>
      )}
    </div>
  );
}
