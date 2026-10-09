'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter, useSearchParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AdminGenreInput, ApiError } from '@bookhaven/api-client';
import { DASHBOARD_QUERY_KEY } from '@/entities/dashboard';
import { adminGenresApi } from '@/entities/genre';
import { GenreForm, genreMutationErrorText, genreToInput } from '@/features/manage-genre';
import { safeBackHref, setFlash } from '@/shared/lib';
import { EditPageHeader } from '@/shared/ui';

// Standalone genre edit page (docs/features/catalog-unified-lists.md).
// «Назад», «Отмена» and the post-save navigation all go to the same
// safeBackHref-resolved list URL.

const ALLOWED_BACK = ['/catalog/genres', '/moderation/genres'];
const FALLBACK_BACK = '/catalog/genres';
const TITLE = 'Редактирование жанра';

export function GenreEditView() {
  const params = useParams();
  const id = params.id as string;
  const router = useRouter();
  const searchParams = useSearchParams();
  const backHref = safeBackHref(searchParams.get('back'), ALLOWED_BACK, FALLBACK_BACK);
  const queryClient = useQueryClient();

  const { data: genre, isLoading, error } = useQuery({
    queryKey: ['admin', 'genres', id],
    queryFn: () => adminGenresApi.get(id),
  });

  const [formData, setFormData] = useState<AdminGenreInput | null>(null);

  // Initialise the form once from the loaded genre.
  useEffect(() => {
    if (genre && formData === null) {
      setFormData(genreToInput(genre));
    }
  }, [genre, formData]);

  const updateMutation = useMutation({
    mutationFn: (data: AdminGenreInput) => adminGenresApi.update(id, data),
    onSuccess: (updated) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'genres'] });
      queryClient.invalidateQueries({ queryKey: DASHBOARD_QUERY_KEY });
      setFlash('catalog.genres', `Жанр «${updated.name}» сохранён.`);
      router.push(backHref as any);
    },
  });

  if (isLoading) {
    return (
      <div className="space-y-6">
        <EditPageHeader backHref={backHref} title={TITLE} />
        <div className="h-64 animate-pulse rounded-lg bg-muted" data-testid="edit-loading" />
      </div>
    );
  }

  if (error || !genre) {
    const notFound = error instanceof ApiError && (error.status === 404 || error.status === 400);
    return (
      <div className="space-y-6">
        <EditPageHeader backHref={backHref} title={TITLE} />
        <div className="rounded-md bg-destructive/10 p-6 text-destructive">
          {notFound ? 'Жанр не найден.' : 'Не удалось загрузить данные.'}
        </div>
      </div>
    );
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (formData) {
      updateMutation.mutate(formData);
    }
  };

  const mutationError = genreMutationErrorText(updateMutation.error);

  return (
    <div className="space-y-6">
      <EditPageHeader backHref={backHref} title={TITLE} subtitle={genre.name} />
      {formData && (
        <GenreForm
          formData={formData}
          onFieldChange={(patch) => setFormData((prev) => (prev ? { ...prev, ...patch } : prev))}
          editing={genre}
          onSubmit={handleSubmit}
          onCancel={() => router.push(backHref as any)}
          pending={updateMutation.isPending}
        />
      )}
      {mutationError && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{mutationError}</div>
      )}
    </div>
  );
}
