'use client';

import {
  formatReadingLine,
  toLocalDateInput,
  todayLocalDateInput,
} from '@/shared/lib/format-reading-history';
import type { BookRead, UpdateReadInput } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { useId, useState } from 'react';
import { useReadingHistory } from '../model/use-reading-history';

interface ReadingHistoryProps {
  bookId: string;
}

interface FormState {
  /** The edited reading id, or 'new' for «Добавить прочтение». */
  target: string;
  start: string;
  end: string;
}

const RANGE_ERROR = 'Дата окончания не может быть раньше даты начала';
const SAVE_ERROR = 'Не удалось сохранить прочтение';
const DELETE_ERROR = 'Не удалось удалить прочтение';
const DELETE_CONFIRM = 'Удалить это прочтение из истории? Статус книги не изменится.';

function endOf(read: BookRead): string | null {
  return read.state === 'finished' ? read.finishedAt : read.abandonedAt;
}

/**
 * Owner-only reading history of a book: an expandable list of the reader's
 * attempts with edit, delete and «Добавить прочтение». Dates are plain days.
 */
export function ReadingHistory({ bookId }: ReadingHistoryProps) {
  const [expanded, setExpanded] = useState(false);
  const [form, setForm] = useState<FormState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const panelId = useId();
  const today = todayLocalDateInput();
  const { reads, isLoading, isError, createRead, updateRead, deleteRead, isSaving, isDeleting } =
    useReadingHistory(bookId, expanded);

  const editedRead =
    form && form.target !== 'new' ? reads.find((r) => r.id === form.target) : undefined;

  function closeForm() {
    setError(null);
    setForm(null);
  }

  function openEdit(read: BookRead) {
    const end = endOf(read);
    setError(null);
    setForm({
      target: read.id,
      start: read.startedAt ? toLocalDateInput(read.startedAt) : '',
      end: end ? toLocalDateInput(end) : '',
    });
  }

  function openAdd() {
    setError(null);
    setForm({ target: 'new', start: '', end: '' });
  }

  async function save() {
    if (!form) return;
    if (form.start && form.end && form.end < form.start) {
      setError(RANGE_ERROR);
      return;
    }
    try {
      if (form.target === 'new') {
        if (!form.end) {
          setError('Укажите дату окончания');
          return;
        }
        await createRead({ startedOn: form.start || null, finishedOn: form.end });
      } else if (editedRead) {
        const initialStart = editedRead.startedAt ? toLocalDateInput(editedRead.startedAt) : '';
        const endIso = endOf(editedRead);
        const initialEnd = endIso ? toLocalDateInput(endIso) : '';
        const input: UpdateReadInput = {};
        if (form.start !== initialStart) {
          if (!form.start && editedRead.state === 'open') {
            setError('Укажите дату начала');
            return;
          }
          input.startedOn = form.start || null;
        }
        if (editedRead.state !== 'open' && form.end !== initialEnd) {
          if (!form.end) {
            setError('Укажите дату окончания');
            return;
          }
          if (editedRead.state === 'finished') input.finishedOn = form.end;
          else input.abandonedOn = form.end;
        }
        if (Object.keys(input).length > 0) await updateRead(editedRead.id, input);
      }
      closeForm();
    } catch {
      setError(SAVE_ERROR);
    }
  }

  async function remove(read: BookRead) {
    if (!window.confirm(DELETE_CONFIRM)) return;
    try {
      setError(null);
      await deleteRead(read.id);
    } catch {
      setError(DELETE_ERROR);
    }
  }

  return (
    <div className="text-sm">
      <button
        type="button"
        className="inline-flex items-center gap-1 text-muted-foreground hover:text-foreground"
        aria-expanded={expanded}
        aria-controls={panelId}
        onClick={() => setExpanded((v) => !v)}
      >
        {expanded ? (
          <ChevronDown className="h-4 w-4" aria-hidden="true" />
        ) : (
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        )}
        История чтения
      </button>

      {expanded && (
        <div id={panelId} className="mt-2 space-y-2">
          {isLoading && <p className="text-muted-foreground">Загрузка...</p>}
          {isError && <p className="text-destructive">Не удалось загрузить историю чтения</p>}
          {!isLoading && !isError && reads.length === 0 && (
            <p className="text-muted-foreground">Вы ещё не отмечали прочтения этой книги.</p>
          )}

          <ul className="space-y-2">
            {reads.map((read, i) => (
              <li key={read.id} className="space-y-2">
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                  <span>{formatReadingLine(i + 1, read)}</span>
                  <Button
                    variant="link"
                    size="sm"
                    className="h-auto p-0"
                    onClick={() => openEdit(read)}
                  >
                    Изменить
                  </Button>
                  {read.state !== 'open' && (
                    <Button
                      variant="link"
                      size="sm"
                      className="h-auto p-0 text-destructive"
                      disabled={isDeleting}
                      onClick={() => void remove(read)}
                    >
                      Удалить
                    </Button>
                  )}
                </div>
                {form?.target === read.id && (
                  <HistoryForm
                    form={form}
                    today={today}
                    read={read}
                    saving={isSaving}
                    onChange={setForm}
                    onSave={() => void save()}
                    onCancel={closeForm}
                  />
                )}
              </li>
            ))}
          </ul>

          {form?.target === 'new' ? (
            <HistoryForm
              form={form}
              today={today}
              saving={isSaving}
              onChange={setForm}
              onSave={() => void save()}
              onCancel={closeForm}
            />
          ) : (
            !isError && (
              <Button variant="outline" size="sm" onClick={openAdd}>
                Добавить прочтение
              </Button>
            )
          )}

          {error && (
            <p role="alert" className="text-destructive">
              {error}
            </p>
          )}
        </div>
      )}
    </div>
  );
}

interface HistoryFormProps {
  form: FormState;
  today: string;
  /** Absent when adding. */
  read?: BookRead;
  saving: boolean;
  onChange: (form: FormState) => void;
  onSave: () => void;
  onCancel: () => void;
}

function HistoryForm({ form, today, read, saving, onChange, onSave, onCancel }: HistoryFormProps) {
  const startId = useId();
  const endId = useId();
  const isOpen = read?.state === 'open';
  const endLabel = read?.state === 'abandoned' ? 'Брошено' : 'Окончание';

  return (
    <div className="flex flex-wrap items-end gap-3 rounded-md border p-3">
      <div className="space-y-1">
        <label htmlFor={startId} className="block text-xs text-muted-foreground">
          Начало
        </label>
        <input
          id={startId}
          type="date"
          className="rounded-md border bg-background px-2 py-1"
          max={today}
          value={form.start}
          onChange={(e) => onChange({ ...form, start: e.target.value })}
        />
      </div>
      {!isOpen && (
        <div className="space-y-1">
          <label htmlFor={endId} className="block text-xs text-muted-foreground">
            {endLabel}
          </label>
          <input
            id={endId}
            type="date"
            className="rounded-md border bg-background px-2 py-1"
            max={today}
            value={form.end}
            onChange={(e) => onChange({ ...form, end: e.target.value })}
          />
        </div>
      )}
      <Button size="sm" disabled={saving} onClick={onSave}>
        {read ? 'Сохранить' : 'Добавить'}
      </Button>
      <Button size="sm" variant="ghost" onClick={onCancel}>
        Отмена
      </Button>
    </div>
  );
}
