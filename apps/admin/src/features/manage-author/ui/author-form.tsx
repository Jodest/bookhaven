'use client';

import { AUTHOR_STATUS_LABELS, PhotoAttributionText } from '@/entities/author';
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
  errorMessage?: string;
}

export function AuthorForm({
  formData,
  onFieldChange,
  editing,
  onSubmit,
  onCancel,
  pending,
  errorMessage,
}: AuthorFormProps) {
  const attribution = editing?.photo_attribution ?? null;
  // A changed photo URL drops the Commons attribution on save
  // (docs/features/author-wikidata-enrichment.md rule 7).
  const photoChanged =
    !!attribution && (formData.photo_url ?? '').trim() !== (editing?.photo_url ?? '').trim();
  // The scheduled harvest queries Wikidata by QID, so the flag needs one
  // (the server answers 422 otherwise).
  const hasQid = editing?.external_source === 'wikidata' && !!editing?.external_id;

  return (
    <form onSubmit={onSubmit} className="space-y-4 rounded-lg border p-4">
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
          {attribution && <PhotoAttributionText attribution={attribution} />}
          {photoChanged && (
            <p className="text-xs text-amber-700">При смене фото атрибуция будет удалена</p>
          )}
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
      {/* Scheduled-harvest opt-in: existing authors only (the server
          ignores it on create). */}
      {editing && (
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="auto_harvest"
              className="h-4 w-4"
              checked={formData.auto_harvest ?? editing.auto_harvest}
              disabled={!hasQid}
              onChange={(e) => onFieldChange({ auto_harvest: e.target.checked })}
            />
            <Label htmlFor="auto_harvest">Автоматически пополнять книги из Wikidata</Label>
          </div>
          {!hasQid && <p className="text-xs text-muted-foreground">Нужен Wikidata QID</p>}
          <p className="text-xs text-muted-foreground">
            {editing.last_auto_harvested_at
              ? `Последний автоматический харвест: ${new Date(editing.last_auto_harvested_at).toLocaleDateString('ru-RU')}`
              : 'Ещё не выполнялся'}
          </p>
        </div>
      )}
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
      {errorMessage && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {errorMessage}
        </div>
      )}
    </form>
  );
}
