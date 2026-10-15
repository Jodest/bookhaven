'use client';

import { useState } from 'react';
import type { AdminScreenWorkDetail } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import {
  formToInput,
  saveWorkErrorText,
  ScreenWorkFields,
  validateWorkForm,
  workToForm,
  type WorkFormState,
} from '@/entities/screen-work';
import { useUpdateScreenWork } from '../model/use-screen-work-mutations';

interface ScreenWorkEditFormProps {
  work: AdminScreenWorkDetail;
  onSaved: (updated: AdminScreenWorkDetail) => void;
  onCancel: () => void;
}

// Edit form of a screen work (openspec screen-works/admin-editing): checks
// the fields before sending, reduces pasted IMDb / Kinopoisk links to ids,
// keeps a stored QID read-only.
export function ScreenWorkEditForm({ work, onSaved, onCancel }: ScreenWorkEditFormProps) {
  const [form, setForm] = useState<WorkFormState>(() => workToForm(work));
  const [error, setError] = useState<string | null>(null);
  const mutation = useUpdateScreenWork(work.id);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const invalid = validateWorkForm(form);
    if (invalid) {
      setError(invalid);
      return;
    }
    setError(null);
    mutation.mutate(formToInput(form), {
      onSuccess: (updated) => onSaved(updated),
      onError: (err) => setError(saveWorkErrorText(err)),
    });
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4 rounded-lg border p-4">
      <ScreenWorkFields
        value={form}
        onChange={(patch) => setForm((prev) => ({ ...prev, ...patch }))}
        qidReadOnly={work.qid !== null}
        idPrefix="edit-work"
      />
      {error && (
        <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}
      <div className="flex gap-2">
        <Button type="submit" disabled={mutation.isPending}>
          Сохранить
        </Button>
        <Button type="button" variant="outline" onClick={onCancel}>
          Отмена
        </Button>
      </div>
    </form>
  );
}
