'use client';

import { Button, Input, Label, Select, Textarea } from '@bookhaven/ui';
import type { CollectionFormValues } from '../lib/collection-form';

interface CollectionFormProps {
  values: CollectionFormValues;
  onChange: (patch: Partial<CollectionFormValues>) => void;
  /** Published genres to link; '' = none. */
  genres: { id: string; name: string }[];
  editing: boolean;
  onSubmit: (e: React.FormEvent) => void;
  pending: boolean;
  /** Field error under «Адрес (slug)». */
  slugError?: string;
}

export function CollectionForm({
  values,
  onChange,
  genres,
  editing,
  onSubmit,
  pending,
  slugError,
}: CollectionFormProps) {
  const genreOptions = [{ value: '', label: 'Без жанра' }, ...genres.map((g) => ({ value: g.id, label: g.name }))];

  return (
    <form onSubmit={onSubmit} className="space-y-4 rounded-lg border p-4">
      <div className="space-y-2">
        <Label htmlFor="title">Название</Label>
        <Input id="title" value={values.title} onChange={(e) => onChange({ title: e.target.value })} required />
      </div>
      <div className="space-y-2">
        <Label htmlFor="slug">Адрес (slug)</Label>
        <Input
          id="slug"
          value={values.slug}
          onChange={(e) => onChange({ slug: e.target.value })}
          placeholder={editing ? undefined : 'Автоматически из названия'}
          aria-invalid={Boolean(slugError)}
        />
        {slugError ? (
          <p className="text-xs text-destructive">{slugError}</p>
        ) : (
          <p className="text-xs text-muted-foreground">
            Страница откроется по адресу /top/{values.slug || '…'}. Латиница, цифры и дефисы.
          </p>
        )}
      </div>
      <div className="space-y-2">
        <Label htmlFor="description">Описание</Label>
        <Textarea
          id="description"
          rows={4}
          value={values.description}
          onChange={(e) => onChange({ description: e.target.value })}
        />
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="size">Количество мест</Label>
          <Input
            id="size"
            type="number"
            min={1}
            max={1000}
            value={Number.isNaN(values.size) ? '' : values.size}
            onChange={(e) => onChange({ size: e.target.value === '' ? NaN : Number(e.target.value) })}
            required
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="genre">Жанр</Label>
          <Select
            id="genre"
            options={genreOptions}
            value={values.genre_id}
            onChange={(e) => onChange({ genre_id: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="source">Источник</Label>
          <Input
            id="source"
            value={values.source}
            onChange={(e) => onChange({ source: e.target.value })}
            placeholder="по версии BBC Big Read, 2003"
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="source_url">Ссылка на источник</Label>
          <Input
            id="source_url"
            type="url"
            value={values.source_url}
            onChange={(e) => onChange({ source_url: e.target.value })}
            placeholder="https://"
          />
        </div>
      </div>
      <div className="flex items-center gap-2">
        <input
          id="is_published"
          type="checkbox"
          checked={values.is_published}
          onChange={(e) => onChange({ is_published: e.target.checked })}
        />
        <Label htmlFor="is_published">Опубликована</Label>
      </div>
      <Button type="submit" disabled={pending}>
        {editing ? 'Сохранить' : 'Создать'}
      </Button>
    </form>
  );
}
