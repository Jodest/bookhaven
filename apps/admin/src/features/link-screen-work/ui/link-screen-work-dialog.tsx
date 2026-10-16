'use client';

import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Button, Input, Label, Select } from '@bookhaven/ui';
import {
  adminScreenWorksApi,
  EMPTY_WORK_FORM,
  formToInput,
  kindLabel,
  linkWorkErrorText,
  ScreenWorkFields,
  validateWorkForm,
  type WorkFormState,
} from '@/entities/screen-work';
import { linkFallbackText, linkSuccessText } from '../lib/link-text';
import { useLinkScreenWork, type LinkTarget } from '../model/use-link-screen-work';

// «Добавить экранизацию» / «Добавить фильм об авторе» dialog (openspec
// screen-works/admin-editing): pick a stored work or create a new one.

const SEARCH_DEBOUNCE_MS = 300;
const MAX_RESULTS = 10;

const RELATION_OPTIONS = [
  { value: 'biopic', label: 'Биографический' },
  { value: 'documentary', label: 'Документальный' },
];

interface LinkScreenWorkDialogProps {
  target: LinkTarget;
  onClose: () => void;
  /** Called with the success banner text after the dialog's request succeeded. */
  onAdded: (message: string) => void;
}

type Mode = 'pick' | 'create';

export function LinkScreenWorkDialog({ target, onClose, onAdded }: LinkScreenWorkDialogProps) {
  const [mode, setMode] = useState<Mode>('pick');
  const [search, setSearch] = useState('');
  const [debounced, setDebounced] = useState('');
  const [form, setForm] = useState<WorkFormState>(EMPTY_WORK_FORM);
  const [relation, setRelation] = useState<'biopic' | 'documentary'>('biopic');
  const [error, setError] = useState<string | null>(null);
  const mutation = useLinkScreenWork(target);

  useEffect(() => {
    const t = setTimeout(() => setDebounced(search.trim()), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [search]);

  const { data, isFetching, isError } = useQuery({
    queryKey: ['admin', 'screen-works', 'link-search', debounced],
    queryFn: () => adminScreenWorksApi.list({ search: debounced, limit: MAX_RESULTS }),
    enabled: mode === 'pick' && debounced !== '',
  });

  const fail = (err: unknown) => setError(linkWorkErrorText(err, linkFallbackText(target.type)));

  const handlePick = (workId: string) => {
    setError(null);
    mutation.mutate(
      { body: { screen_work_id: workId }, relation },
      {
        onSuccess: (item) => {
          onAdded(linkSuccessText(target.type, item.title, false));
          onClose();
        },
        onError: fail,
      }
    );
  };

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault();
    const invalid = validateWorkForm(form);
    if (invalid) {
      setError(invalid);
      return;
    }
    setError(null);
    mutation.mutate(
      { body: { work: formToInput(form) }, relation },
      {
        onSuccess: (item) => {
          onAdded(linkSuccessText(target.type, item.title, !item.work_created));
          onClose();
        },
        onError: fail,
      }
    );
  };

  const title = target.type === 'book' ? 'Добавить экранизацию' : 'Добавить фильм об авторе';
  const results = data?.items ?? [];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="link-screen-work-title"
        className="max-h-[90vh] w-full max-w-2xl space-y-4 overflow-y-auto rounded-lg bg-background p-6 shadow-lg"
      >
        <h2 id="link-screen-work-title" className="text-lg font-semibold">
          {title}
        </h2>

        <div className="flex gap-2">
          <Button
            type="button"
            size="sm"
            variant={mode === 'pick' ? 'default' : 'outline'}
            onClick={() => {
              setMode('pick');
              setError(null);
            }}
          >
            Выбрать из каталога
          </Button>
          <Button
            type="button"
            size="sm"
            variant={mode === 'create' ? 'default' : 'outline'}
            onClick={() => {
              setMode('create');
              setError(null);
            }}
          >
            Создать новый
          </Button>
        </div>

        {target.type === 'author' && (
          <div className="space-y-2">
            <Label htmlFor="link-relation">Тип связи</Label>
            <Select
              id="link-relation"
              options={RELATION_OPTIONS}
              value={relation}
              onChange={(e) => setRelation(e.target.value as 'biopic' | 'documentary')}
            />
          </div>
        )}

        {mode === 'pick' ? (
          <div className="space-y-3">
            <Input
              aria-label="Поиск фильма"
              placeholder="Поиск по названию, QID, IMDb или Кинопоиску"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            {isError && <p className="text-sm text-destructive">Не удалось загрузить фильмы.</p>}
            {debounced !== '' && !isFetching && !isError && results.length === 0 && (
              <p className="text-sm text-muted-foreground">Фильмы не найдены.</p>
            )}
            {results.length > 0 && (
              <ul className="divide-y rounded-md border">
                {results.map((work) => {
                  const linked = target.linkedIds.includes(work.id);
                  return (
                    <li key={work.id} className="flex flex-wrap items-center justify-between gap-2 p-2">
                      <div className="min-w-0 text-sm">
                        <div className="font-medium">{work.title}</div>
                        <div className="text-muted-foreground">
                          {[work.year ? String(work.year) : null, kindLabel(work.kind), work.qid]
                            .filter(Boolean)
                            .join(' · ')}
                        </div>
                      </div>
                      {linked ? (
                        <span className="text-sm text-muted-foreground">Уже привязан</span>
                      ) : (
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          disabled={mutation.isPending}
                          onClick={() => handlePick(work.id)}
                        >
                          Выбрать
                        </Button>
                      )}
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        ) : (
          <form onSubmit={handleCreate} className="space-y-4">
            <ScreenWorkFields
              value={form}
              onChange={(patch) => setForm((prev) => ({ ...prev, ...patch }))}
              idPrefix="new-work"
            />
            <Button type="submit" disabled={mutation.isPending}>
              Добавить
            </Button>
          </form>
        )}

        {error && (
          <div role="alert" className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
            {error}
          </div>
        )}

        <div className="flex justify-end">
          <Button type="button" variant="ghost" onClick={onClose}>
            Отмена
          </Button>
        </div>
      </div>
    </div>
  );
}
