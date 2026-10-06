'use client';

import { AUTHOR_STATUS_LABELS } from '@/entities/author';
import { AdminAuthor, AdminAuthorInput, AdminAuthorStatus } from '@bookhaven/api-client';
import { Button, Input, Label, Select } from '@bookhaven/ui';

const STATUS_OPTIONS = (['pending', 'published', 'rejected'] as const).map((value) => ({
  value,
  label: AUTHOR_STATUS_LABELS[value],
}));

interface AuthorFormProps {
  formData: AdminAuthorInput;
  onFieldChange: (patch: Partial<AdminAuthorInput>) => void;
  editing: AdminAuthor | null;
  onSubmit: (e: React.FormEvent) => void;
  onCancel: () => void;
  pending: boolean;
  successMessage: string | null;
  onDismissSuccess: () => void;
  errorMessage?: string;
}

export function AuthorForm({
  formData,
  onFieldChange,
  editing,
  onSubmit,
  onCancel,
  pending,
  successMessage,
  onDismissSuccess,
  errorMessage,
}: AuthorFormProps) {
  return (
    <form onSubmit={onSubmit} className="space-y-4 rounded-lg border p-4">
      <h2 className="font-semibold">
        {editing ? 'Редактировать автора' : 'Новый автор'}
      </h2>
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="name">Имя *</Label>
          <Input
            id="name"
            value={formData.name}
            onChange={(e) => onFieldChange({ name: e.target.value })}
            required
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="slug">Slug</Label>
          <Input
            id="slug"
            value={formData.slug || ''}
            onChange={(e) => onFieldChange({ slug: e.target.value })}
            placeholder="Автоматически из имени"
          />
        </div>
        {/* Status is editable on existing authors only; new authors are
            always created published by the server. */}
        {editing && (
          <div className="space-y-2">
            <Label htmlFor="status">Статус</Label>
            <Select
              id="status"
              options={STATUS_OPTIONS}
              value={formData.status ?? editing.status}
              onChange={(e) => onFieldChange({ status: e.target.value as AdminAuthorStatus })}
            />
          </div>
        )}
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="photo_url">URL фото</Label>
          <Input
            id="photo_url"
            value={formData.photo_url || ''}
            onChange={(e) => onFieldChange({ photo_url: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="birth_date">Дата рождения</Label>
          <Input
            id="birth_date"
            type="date"
            value={formData.birth_date || ''}
            onChange={(e) => onFieldChange({ birth_date: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="death_date">Дата смерти</Label>
          <Input
            id="death_date"
            type="date"
            value={formData.death_date || ''}
            onChange={(e) => onFieldChange({ death_date: e.target.value })}
          />
        </div>
      </div>
      <div className="space-y-2">
        <Label htmlFor="biography">Биография</Label>
        <Input
          id="biography"
          value={formData.biography || ''}
          onChange={(e) => onFieldChange({ biography: e.target.value })}
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
      {successMessage && (
        <div className="rounded-md bg-green-100 p-3 text-sm text-green-900">
          {successMessage}
          <button type="button" onClick={onDismissSuccess} className="ml-4 underline">
            Скрыть
          </button>
        </div>
      )}
      {errorMessage && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {errorMessage}
        </div>
      )}
    </form>
  );
}
