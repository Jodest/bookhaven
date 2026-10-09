'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter, useSearchParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AdminAuthorInput, ApiError } from '@bookhaven/api-client';
import { adminAuthorsApi } from '@/entities/author';
import { DASHBOARD_QUERY_KEY } from '@/entities/dashboard';
import { AuthorForm, authorToInput } from '@/features/manage-author';
import { safeBackHref, setFlash } from '@/shared/lib';
import { EditPageHeader } from '@/shared/ui';

// Standalone author edit page (docs/features/catalog-unified-lists.md).
// «Назад», «Отмена» and the post-save navigation all go to the same
// safeBackHref-resolved list URL.

const ALLOWED_BACK = ['/catalog/authors', '/moderation/authors'];
const FALLBACK_BACK = '/catalog/authors';
const TITLE = 'Редактирование автора';

export function AuthorEditView() {
  const params = useParams();
  const id = params.id as string;
  const router = useRouter();
  const searchParams = useSearchParams();
  const backHref = safeBackHref(searchParams.get('back'), ALLOWED_BACK, FALLBACK_BACK);
  const queryClient = useQueryClient();

  const { data: author, isLoading, error } = useQuery({
    queryKey: ['admin', 'authors', id],
    queryFn: () => adminAuthorsApi.get(id),
  });

  const [formData, setFormData] = useState<AdminAuthorInput | null>(null);

  // Initialise the form once from the loaded author.
  useEffect(() => {
    if (author && formData === null) {
      setFormData(authorToInput(author));
    }
  }, [author, formData]);

  const updateMutation = useMutation({
    mutationFn: (data: AdminAuthorInput) => adminAuthorsApi.update(id, data),
    onSuccess: (updated) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
      queryClient.invalidateQueries({ queryKey: DASHBOARD_QUERY_KEY });
      setFlash('catalog.authors', `Автор «${updated.name}» сохранён.`);
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

  if (error || !author) {
    const notFound = error instanceof ApiError && (error.status === 404 || error.status === 400);
    return (
      <div className="space-y-6">
        <EditPageHeader backHref={backHref} title={TITLE} />
        <div className="rounded-md bg-destructive/10 p-6 text-destructive">
          {notFound ? 'Автор не найден.' : 'Не удалось загрузить данные.'}
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

  return (
    <div className="space-y-6">
      <EditPageHeader backHref={backHref} title={TITLE} subtitle={author.name} />
      {formData && (
        <AuthorForm
          formData={formData}
          onFieldChange={(patch) => setFormData((prev) => (prev ? { ...prev, ...patch } : prev))}
          editing={author}
          onSubmit={handleSubmit}
          onCancel={() => router.push(backHref as any)}
          pending={updateMutation.isPending}
          errorMessage={updateMutation.error?.message}
        />
      )}
    </div>
  );
}
