'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { adminAuthorsApi } from '@/entities/author';
import { adminGenresApi } from '@/entities/genre';
import { AdminBook, AdminBookInput, ApiError } from '@bookhaven/api-client';
import {
  Button,
  Input,
  Label,
  Textarea,
  Select,
} from '@bookhaven/ui';
import { useForm } from 'react-hook-form';
import { useRouter } from 'next/navigation';
import { MultiSelect } from '@/shared/ui';

interface BookFormProps {
  initial?: AdminBook;
  onSubmit: (data: AdminBookInput) => Promise<unknown>;
}

export function BookForm({ initial, onSubmit }: BookFormProps) {
  const router = useRouter();
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    setValue,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<AdminBookInput>({
    defaultValues: initial
      ? {
          title: initial.title,
          slug: initial.slug,
          description: initial.description,
          cover_url: initial.cover_url,
          isbn: initial.isbn,
          published_year: initial.published_year,
          page_count: initial.page_count,
          language: initial.language,
          author_ids: initial.authors.map((a) => a.id),
          genre_ids: initial.genres.map((g) => g.id),
          is_hidden: initial.is_hidden,
        }
      : {
          language: 'ru',
          author_ids: [],
          genre_ids: [],
          is_hidden: false,
        },
  });

  const authorIds = watch('author_ids');
  const genreIds = watch('genre_ids');

  const { data: authorsData } = useQuery({
    queryKey: ['admin', 'authors', 'all'],
    queryFn: () => adminAuthorsApi.list({ limit: 100 }),
  });

  const { data: genresData } = useQuery({
    queryKey: ['admin', 'genres', 'all'],
    queryFn: () => adminGenresApi.list({ limit: 100 }),
  });

  const authorOptions =
    authorsData?.items.map((a) => ({ value: a.id, label: a.name })) || [];
  const genreOptions =
    genresData?.items.map((g) => ({ value: g.id, label: g.name })) || [];

  const handleFormSubmit = async (data: AdminBookInput) => {
    setServerError(null);

    if (initial && initial.is_hidden !== data.is_hidden) {
      const action = data.is_hidden ? 'скрыть' : 'вернуть в видимость';
      if (!window.confirm(`Вы уверены, что хотите ${action} эту книгу?`)) {
        return;
      }
    }

    try {
      await onSubmit(data);
      router.push('/books' as any);
    } catch (error) {
      let message = 'Не удалось сохранить книгу. Проверьте введенные данные.';
      if (error instanceof ApiError) {
        if (error.status === 409) {
          message = 'Книга с таким slug уже существует.';
        } else if (error.status === 422 && error.message) {
          message = error.message;
        } else if (error.status >= 500) {
          message = 'Ошибка сервера. Попробуйте позже.';
        }
      }
      setServerError(message);
    }
  };

  return (
    <form onSubmit={handleSubmit(handleFormSubmit)} className="max-w-2xl space-y-6">
      {serverError && (
        <div className="rounded-md bg-destructive/10 p-4 text-sm text-destructive">
          {serverError}
        </div>
      )}

      <div className="space-y-2">
        <Label htmlFor="title">Название *</Label>
        <Input id="title" {...register('title', { required: 'Обязательное поле' })} />
        {errors.title && <p className="text-sm text-destructive">{errors.title.message}</p>}
      </div>

      <div className="space-y-2">
        <Label htmlFor="slug">Slug</Label>
        <Input id="slug" {...register('slug')} placeholder="Автоматически из названия" />
      </div>

      <div className="space-y-2">
        <Label htmlFor="description">Описание</Label>
        <Textarea id="description" {...register('description')} rows={5} />
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="isbn">ISBN</Label>
          <Input id="isbn" {...register('isbn')} />
        </div>
        <div className="space-y-2">
          <Label htmlFor="language">Язык *</Label>
          <Input id="language" {...register('language', { required: 'Обязательное поле' })} />
          {errors.language && <p className="text-sm text-destructive">{errors.language.message}</p>}
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="published_year">Год издания</Label>
          <Input
            id="published_year"
            type="number"
            {...register('published_year', { valueAsNumber: true })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="page_count">Количество страниц</Label>
          <Input
            id="page_count"
            type="number"
            {...register('page_count', { valueAsNumber: true })}
          />
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="cover_url">URL обложки</Label>
        <Input id="cover_url" {...register('cover_url')} />
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label>Авторы</Label>
          <MultiSelect
            options={authorOptions}
            value={authorIds || []}
            onChange={(v) => setValue('author_ids', v)}
          />
        </div>
        <div className="space-y-2">
          <Label>Жанры</Label>
          <MultiSelect
            options={genreOptions}
            value={genreIds || []}
            onChange={(v) => setValue('genre_ids', v)}
          />
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="is_hidden">Видимость</Label>
        <Select
          id="is_hidden"
          options={[
            { value: 'false', label: 'Видима' },
            { value: 'true', label: 'Скрыта' },
          ]}
          value={watch('is_hidden') ? 'true' : 'false'}
          onChange={(e) => setValue('is_hidden', e.target.value === 'true')}
        />
      </div>

      <div className="flex gap-4">
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? 'Сохранение...' : initial ? 'Сохранить' : 'Создать'}
        </Button>
        <Button type="button" variant="outline" onClick={() => router.push('/books' as any)}>
          Отмена
        </Button>
      </div>
    </form>
  );
}
