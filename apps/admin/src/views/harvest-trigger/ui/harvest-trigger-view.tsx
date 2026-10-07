'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import { adminHarvestApi, HARVEST_JOBS_QUERY_KEY } from '@/entities/harvest';
import { ApiError } from '@bookhaven/api-client';
import { Button, Input, Label, Textarea } from '@bookhaven/ui';
import { AuthorNameSearch, SelectedAuthor } from './author-name-search';

type AuthorInputMode = 'search' | 'manual';

// Parses the free-text QID field: one per line or comma-separated (both
// accepted), trimmed, blanks dropped, deduped client-side. Format
// (^Q[0-9]+$) validation itself happens server-side in
// bookdata.BuildAuthorListQuery - this is just splitting.
function parseAuthorQIDs(value: string): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const raw of value.split(/[,\n]/)) {
    const qid = raw.trim();
    if (!qid || seen.has(qid)) continue;
    seen.add(qid);
    result.push(qid);
  }
  return result;
}

/**
 * Harvest trigger page (/import/wikidata, docs/features/admin-navigation-restructure.md
 * phase 4). Does not show the jobs list: on success it shows an inline
 * confirmation with a link to the Задачи tab (/import/jobs).
 */
export function HarvestTriggerView() {
  const [mode, setMode] = useState<AuthorInputMode>('search');
  const [selectedAuthors, setSelectedAuthors] = useState<SelectedAuthor[]>([]);
  const [qidsText, setQidsText] = useState('');
  const [limitText, setLimitText] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<{ taskId: string } | null>(null);
  const queryClient = useQueryClient();

  // Switching modes discards the *other* mode's in-progress input, per
  // docs/features/harvest-author-search.md Business Rules - the two modes
  // are mutually exclusive per submit, never merged.
  const handleModeChange = (nextMode: AuthorInputMode) => {
    if (nextMode === mode) return;
    if (nextMode === 'manual') {
      setSelectedAuthors([]);
    } else {
      setQidsText('');
    }
    setMode(nextMode);
    setError(null);
    setSuccess(null);
  };

  const handleAddAuthor = (author: SelectedAuthor) => {
    setSelectedAuthors((prev) => (prev.some((a) => a.qid === author.qid) ? prev : [...prev, author]));
  };

  const handleRemoveAuthor = (qid: string) => {
    setSelectedAuthors((prev) => prev.filter((a) => a.qid !== qid));
  };

  const triggerMutation = useMutation({
    mutationFn: adminHarvestApi.trigger,
    onSuccess: (res) => {
      setError(null);
      setQidsText('');
      setSelectedAuthors([]);
      setLimitText('');
      setSuccess({ taskId: res.task_id });
      queryClient.invalidateQueries({ queryKey: HARVEST_JOBS_QUERY_KEY });
    },
    onError: (err) => {
      let message = 'Не удалось запустить харвест. Попробуйте снова.';
      if (err instanceof ApiError && err.message) {
        message = err.message;
      }
      setError(message);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setSuccess(null);

    // The two modes are mutually exclusive per submit - author_qids comes
    // from exactly one of selectedAuthors or the parsed QID textarea, never
    // a merge of both.
    let authorQids: string[];
    if (mode === 'search') {
      authorQids = selectedAuthors.map((a) => a.qid);
      if (authorQids.length === 0) {
        setError('Выберите хотя бы одного автора.');
        return;
      }
    } else {
      authorQids = parseAuthorQIDs(qidsText);
      if (authorQids.length === 0) {
        setError('Введите хотя бы один QID автора.');
        return;
      }
    }
    setError(null);

    const limit = limitText.trim() === '' ? undefined : Number(limitText);
    triggerMutation.mutate({
      author_qids: authorQids,
      ...(limit !== undefined && !Number.isNaN(limit) ? { limit } : {}),
    });
  };

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Харвест из Wikidata</h1>

      <form onSubmit={handleSubmit} className="max-w-xl space-y-4 rounded-lg border p-6">
        <div className="flex gap-2">
          <Button
            type="button"
            variant={mode === 'search' ? 'default' : 'outline'}
            size="sm"
            onClick={() => handleModeChange('search')}
          >
            Поиск по имени
          </Button>
          <Button
            type="button"
            variant={mode === 'manual' ? 'default' : 'outline'}
            size="sm"
            onClick={() => handleModeChange('manual')}
          >
            Ввести QID вручную
          </Button>
        </div>

        {mode === 'search' ? (
          <AuthorNameSearch
            selectedAuthors={selectedAuthors}
            onAdd={handleAddAuthor}
            onRemove={handleRemoveAuthor}
          />
        ) : (
          <div className="space-y-2">
            <Label htmlFor="harvest-qids">QID авторов Wikidata</Label>
            <Textarea
              id="harvest-qids"
              value={qidsText}
              onChange={(e) => setQidsText(e.target.value)}
              placeholder={'Q7200\nQ1183296'}
              rows={5}
            />
            <p className="text-xs text-muted-foreground">
              Один QID на строку или через запятую, например Q7200
            </p>
          </div>
        )}

        <div className="space-y-2">
          <Label htmlFor="harvest-limit">Лимит (необязательно)</Label>
          <Input
            id="harvest-limit"
            type="number"
            placeholder="500"
            value={limitText}
            onChange={(e) => setLimitText(e.target.value)}
          />
        </div>

        <Button type="submit" disabled={triggerMutation.isPending}>
          {triggerMutation.isPending ? 'Запуск...' : 'Запустить харвест'}
        </Button>
      </form>

      {success && (
        <div
          role="status"
          className="flex max-w-xl items-start justify-between gap-4 rounded-md bg-green-100 p-3 text-sm text-green-900"
        >
          <div className="space-y-1">
            <div>
              Харвест запущен (задача {success.taskId.slice(0, 12)}). Ход выполнения — на вкладке
              «Задачи».
            </div>
            <Link href={'/import/jobs' as any} className="underline">
              К задачам
            </Link>
          </div>
          <button type="button" onClick={() => setSuccess(null)} className="underline">
            Скрыть
          </button>
        </div>
      )}

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-sm text-destructive">{error}</div>
      )}
    </div>
  );
}
