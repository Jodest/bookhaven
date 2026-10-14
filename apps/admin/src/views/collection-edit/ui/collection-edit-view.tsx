'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { ApiError } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { adminCollectionsApi, collectionDetailKey, shownWarning } from '@/entities/curated-collection';
import { adminGenresApi } from '@/entities/genre';
import {
  CollectionForm,
  EMPTY_COLLECTION_FORM,
  collectionErrorText,
  collectionToForm,
  formToCreateInput,
  formToUpdateInput,
  isSlugConflict,
  useCreateCollection,
  useDeleteCollection,
  useUpdateCollection,
  type CollectionFormValues,
} from '@/features/manage-collection';
import { EditPageHeader } from '@/shared/ui';
import { CollectionBooks } from '@/widgets/collection-books';
import { CollectionSnapshot } from '@/widgets/collection-snapshot';

const BACK_HREF = '/collections';

interface CollectionEditViewProps {
  /** `new` renders only the form and redirects to the edit page after create. */
  mode?: 'new' | 'edit';
}

export function CollectionEditView({ mode = 'edit' }: CollectionEditViewProps) {
  return mode === 'new' ? <NewCollection /> : <EditCollection />;
}

function useGenreOptions(current?: { id: string; name: string } | null) {
  const { data } = useQuery({
    queryKey: ['admin', 'genres', 'collection-options'],
    queryFn: () => adminGenresApi.list({ status: 'published', limit: 100 }),
  });
  const options = (data?.items ?? []).map((g) => ({ id: g.id, name: g.name }));
  if (current && !options.some((g) => g.id === current.id)) options.unshift(current);
  return options;
}

function NewCollection() {
  const router = useRouter();
  const [values, setValues] = useState<CollectionFormValues>(EMPTY_COLLECTION_FORM);
  const genres = useGenreOptions();
  const create = useCreateCollection((created) => router.push(`/collections/${created.id}` as any));

  return (
    <div className="space-y-6">
      <EditPageHeader backHref={BACK_HREF} title="Новая подборка" />
      <CollectionForm
        values={values}
        onChange={(patch) => setValues((prev) => ({ ...prev, ...patch }))}
        genres={genres}
        editing={false}
        pending={create.isPending}
        slugError={isSlugConflict(create.error) ? collectionErrorText(create.error) : undefined}
        onSubmit={(e) => {
          e.preventDefault();
          create.mutate(formToCreateInput(values));
        }}
      />
      {create.error && !isSlugConflict(create.error) && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {collectionErrorText(create.error)}
        </div>
      )}
    </div>
  );
}

function EditCollection() {
  const params = useParams();
  const id = params.id as string;
  const router = useRouter();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [saved, setSaved] = useState(false);

  const { data: collection, isLoading, error } = useQuery({
    queryKey: collectionDetailKey(id),
    queryFn: () => adminCollectionsApi.get(id),
  });

  const [values, setValues] = useState<CollectionFormValues | null>(null);
  useEffect(() => {
    if (collection && values === null) setValues(collectionToForm(collection));
  }, [collection, values]);

  const genres = useGenreOptions(collection?.genre);
  const update = useUpdateCollection(id, () => setSaved(true));
  const remove = useDeleteCollection(id, () => router.push(BACK_HREF as any));

  if (isLoading) {
    return (
      <div className="space-y-6">
        <EditPageHeader backHref={BACK_HREF} title="Редактирование подборки" />
        <div className="h-64 animate-pulse rounded-lg bg-muted" data-testid="edit-loading" />
      </div>
    );
  }

  if (error || !collection) {
    const notFound = error instanceof ApiError && (error.status === 404 || error.status === 400);
    return (
      <div className="space-y-6">
        <EditPageHeader backHref={BACK_HREF} title="Редактирование подборки" />
        <div className="rounded-md bg-destructive/10 p-6 text-destructive">
          {notFound ? 'Подборка не найдена.' : 'Не удалось загрузить данные.'}
        </div>
      </div>
    );
  }

  const warning = shownWarning(collection.kind, collection.shown_count, collection.size);
  const mutationError = update.error ?? remove.error;

  return (
    <div className="space-y-6">
      <EditPageHeader backHref={BACK_HREF} title="Редактирование подборки" subtitle={collection.title} />
      {warning && (
        <div role="alert" className="rounded-md bg-amber-100 p-3 text-sm text-amber-900">
          {warning}
        </div>
      )}
      {values && (
        <CollectionForm
          values={values}
          onChange={(patch) => {
            setSaved(false);
            setValues((prev) => (prev ? { ...prev, ...patch } : prev));
          }}
          genres={genres}
          editing
          pending={update.isPending}
          slugError={isSlugConflict(update.error) ? collectionErrorText(update.error) : undefined}
          onSubmit={(e) => {
            e.preventDefault();
            update.mutate(formToUpdateInput(values));
          }}
        />
      )}
      {saved && <div className="rounded-md bg-muted p-3 text-sm">Подборка сохранена.</div>}
      {mutationError && !isSlugConflict(mutationError) && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {collectionErrorText(mutationError)}
        </div>
      )}

      {collection.kind === 'auto' ? (
        <CollectionSnapshot collection={collection} />
      ) : (
        <CollectionBooks collection={collection} />
      )}

      <div className="border-t pt-4">
        {confirmDelete ? (
          <div className="flex items-center gap-2">
            <span className="text-sm">Удалить подборку «{collection.title}»? Книги каталога останутся.</span>
            <Button type="button" variant="destructive" disabled={remove.isPending} onClick={() => remove.mutate()}>
              Да, удалить
            </Button>
            <Button type="button" variant="outline" onClick={() => setConfirmDelete(false)}>
              Отмена
            </Button>
          </div>
        ) : (
          <Button type="button" variant="outline" onClick={() => setConfirmDelete(true)}>
            Удалить подборку
          </Button>
        )}
      </div>
    </div>
  );
}
