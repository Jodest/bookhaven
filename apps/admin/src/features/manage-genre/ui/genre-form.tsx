'use client';

import { AdminGenre, AdminGenreInput } from '@bookhaven/api-client';
import { Button, Input, Label } from '@bookhaven/ui';

interface GenreFormProps {
  formData: AdminGenreInput;
  onFieldChange: (patch: Partial<AdminGenreInput>) => void;
  editing: AdminGenre | null;
  onSubmit: (e: React.FormEvent) => void;
  onCancel: () => void;
  pending: boolean;
}

export function GenreForm({
  formData,
  onFieldChange,
  editing,
  onSubmit,
  onCancel,
  pending,
}: GenreFormProps) {
  return (
    <form
      onSubmit={onSubmit}
      className="flex flex-col gap-4 rounded-lg border p-4 md:flex-row md:items-end"
    >
      <div className="flex-1 space-y-2">
        <Label htmlFor="name">Название *</Label>
        <Input
          id="name"
          value={formData.name}
          onChange={(e) => onFieldChange({ name: e.target.value })}
          required
        />
      </div>
      <div className="flex-1 space-y-2">
        <Label htmlFor="slug">Slug</Label>
        <Input
          id="slug"
          value={formData.slug || ''}
          onChange={(e) => onFieldChange({ slug: e.target.value })}
          placeholder="Автоматически из названия"
        />
      </div>
      <div className="flex gap-2">
        <Button type="submit" disabled={pending}>
          {editing ? 'Сохранить' : 'Создать'}
        </Button>
        {editing && (
          <Button type="button" variant="outline" onClick={onCancel}>
            Отмена
          </Button>
        )}
      </div>
    </form>
  );
}
