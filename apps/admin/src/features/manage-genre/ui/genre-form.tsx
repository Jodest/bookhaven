'use client';

import { GENRE_STATUS_LABELS } from '@/entities/genre';
import { AdminGenre, AdminGenreInput, AdminGenreStatus } from '@bookhaven/api-client';
import { Button, Input, Label, Select } from '@bookhaven/ui';

const STATUS_OPTIONS = (['pending', 'published', 'rejected'] as const).map((value) => ({
  value,
  label: GENRE_STATUS_LABELS[value],
}));

/** Client-side hint only; the server normalizes (trim, uppercase) and validates. */
const QID_PATTERN = /^Q[0-9]+$/;

function isQIDHintValid(value: string | null | undefined): boolean {
  const v = (value ?? '').trim().toUpperCase();
  return v === '' || QID_PATTERN.test(v);
}

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
  const qidValid = isQIDHintValid(formData.wikidata_qid);

  return (
    <form
      onSubmit={onSubmit}
      className="flex flex-col gap-4 rounded-lg border p-4 md:flex-row md:items-start"
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
      <div className="flex-1 space-y-2">
        <Label htmlFor="wikidata_qid">Wikidata QID</Label>
        <Input
          id="wikidata_qid"
          value={formData.wikidata_qid ?? ''}
          onChange={(e) => onFieldChange({ wikidata_qid: e.target.value })}
          placeholder="Например, Q8261"
          aria-invalid={!qidValid}
        />
        {!qidValid && (
          <p className="text-xs text-amber-700">QID должен иметь вид Q123</p>
        )}
      </div>
      {/* Status is editable on existing genres only; new genres are always
          created published by the server. */}
      {editing && (
        <div className="flex-1 space-y-2">
          <Label htmlFor="status">Статус</Label>
          <Select
            id="status"
            options={STATUS_OPTIONS}
            value={formData.status ?? editing.status}
            onChange={(e) => onFieldChange({ status: e.target.value as AdminGenreStatus })}
          />
        </div>
      )}
      <div className="flex gap-2 md:pt-6">
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
