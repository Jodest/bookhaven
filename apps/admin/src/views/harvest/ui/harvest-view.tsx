'use client';

import { Fragment, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import { adminHarvestApi } from '@/entities/harvest';
import { ApiError, AdminHarvestJob } from '@bookhaven/api-client';
import {
  Badge,
  Button,
  Input,
  Label,
  Progress,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
} from '@bookhaven/ui';
import { AuthorNameSearch, SelectedAuthor } from './author-name-search';

type AuthorInputMode = 'search' | 'manual';

const JOBS_QUERY_KEY = ['admin', 'harvest', 'jobs'];

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

function stateBadgeVariant(state: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (state) {
    case 'active':
      return 'default';
    case 'pending':
    case 'retry':
      return 'secondary';
    case 'archived':
      return 'destructive';
    case 'completed':
      return 'outline';
    default:
      return 'secondary';
  }
}

function jobResultSummary(job: AdminHarvestJob): string {
  if (job.state === 'completed' && job.result) {
    return `создано ${job.result.created}, пропущено ${job.result.skipped_existing}, ошибок ${job.result.errors.length}`;
  }
  if (job.last_error) {
    return job.last_error;
  }
  return '—';
}

// jobResultCell renders the "Результат" table cell for a single job. Only
// an "active" job with a partial result whose total is known (> 0) gets a
// progress bar - everything else (including "retry", whose result if any
// reflects the previous failed attempt, not the upcoming one) keeps the
// existing plain-text summary.
function JobResultCell({ job }: { job: AdminHarvestJob }) {
  if (job.state === 'active') {
    if (!job.result || job.result.total === 0) {
      return <span className="text-muted-foreground">Выполняется…</span>;
    }
    const processed = Math.min(job.result.processed, job.result.total);
    const percent = Math.round((processed / job.result.total) * 100);
    return (
      <div className="space-y-1">
        <Progress value={percent} className="w-32" />
        <div className="text-xs text-muted-foreground">
          {processed} / {job.result.total} ({percent}%)
        </div>
      </div>
    );
  }
  return <>{jobResultSummary(job)}</>;
}

// Fixed label for every skipped-record row: today there is exactly one skip
// cause (idempotency on external_source/external_id), so the backend's
// HarvestSkippedRecord carries no per-record message - see
// docs/features/harvest-job-report.md Business Rules.
const SKIPPED_RECORD_REASON = 'уже есть в каталоге (совпадение по внешнему источнику)';

// JobReportRow renders the expanded per-record detail (errors + skipped
// records) for a single job, immediately below its row in the jobs table.
// Reuses the already-polled GET /jobs list response's result.errors/
// result.skipped_records - no separate GET /jobs/:id fetch (see
// docs/features/harvest-job-report.md Frontend decision).
function JobReportRow({ job }: { job: AdminHarvestJob }) {
  const errors = job.result?.errors ?? [];
  const skippedRecords = job.result?.skipped_records ?? [];

  return (
    <TableRow>
      <TableCell colSpan={4} className="space-y-4 bg-muted/30">
        {errors.length > 0 && (
          <div className="space-y-2">
            <h3 className="text-sm font-semibold">Ошибки</h3>
            <div className="rounded-lg border bg-background">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>№</TableHead>
                    <TableHead>Название</TableHead>
                    <TableHead>Причина</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {errors.map((recordError) => (
                    <TableRow key={`${recordError.index}-${recordError.message}`}>
                      <TableCell>{recordError.index + 1}</TableCell>
                      <TableCell>{recordError.title || '—'}</TableCell>
                      <TableCell>{recordError.message}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </div>
        )}
        {skippedRecords.length > 0 && (
          <div className="space-y-2">
            <h3 className="text-sm font-semibold">Пропущено</h3>
            <div className="rounded-lg border bg-background">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>№</TableHead>
                    <TableHead>Название</TableHead>
                    <TableHead>Причина</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {skippedRecords.map((skipped) => (
                    <TableRow key={`${skipped.index}-${skipped.title}`}>
                      <TableCell>{skipped.index + 1}</TableCell>
                      <TableCell>{skipped.title || '—'}</TableCell>
                      <TableCell>{SKIPPED_RECORD_REASON}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </div>
        )}
      </TableCell>
    </TableRow>
  );
}

export function HarvestView() {
  const [mode, setMode] = useState<AuthorInputMode>('search');
  const [selectedAuthors, setSelectedAuthors] = useState<SelectedAuthor[]>([]);
  const [qidsText, setQidsText] = useState('');
  const [limitText, setLimitText] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [expandedJobIds, setExpandedJobIds] = useState<Set<string>>(new Set());
  const queryClient = useQueryClient();

  const toggleJobExpanded = (id: string) => {
    setExpandedJobIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

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
  };

  const handleAddAuthor = (author: SelectedAuthor) => {
    setSelectedAuthors((prev) => (prev.some((a) => a.qid === author.qid) ? prev : [...prev, author]));
  };

  const handleRemoveAuthor = (qid: string) => {
    setSelectedAuthors((prev) => prev.filter((a) => a.qid !== qid));
  };

  const jobsQuery = useQuery({
    queryKey: JOBS_QUERY_KEY,
    queryFn: adminHarvestApi.listJobs,
    refetchInterval: 5000,
  });

  const triggerMutation = useMutation({
    mutationFn: adminHarvestApi.trigger,
    onSuccess: () => {
      setError(null);
      setQidsText('');
      setSelectedAuthors([]);
      setLimitText('');
      queryClient.invalidateQueries({ queryKey: JOBS_QUERY_KEY });
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

  const jobs = jobsQuery.data?.items ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold tracking-tight">Харвестер</h1>
        <div className="flex gap-2">
          <Link href={'/books?visibility=pending_review' as any}>
            <Button variant="outline">К очереди на проверку</Button>
          </Link>
          <Link href={'/authors/moderation' as any}>
            <Button variant="outline">К авторам на модерации</Button>
          </Link>
        </div>
      </div>

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

      {error && (
        <div className="rounded-md bg-destructive/10 p-4 text-sm text-destructive">{error}</div>
      )}

      <div className="space-y-2">
        <h2 className="text-xl font-semibold">Недавние задачи</h2>
        <p className="text-xs text-muted-foreground">
          Список ограничен состоянием очереди asynq/Redis (до 50 задач на состояние); задачи,
          вышедшие за пределы хранения Redis, здесь больше не отображаются.
        </p>
        <div className="rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Статус</TableHead>
                <TableHead>Результат</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {jobs.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} className="text-center text-muted-foreground">
                    Нет задач
                  </TableCell>
                </TableRow>
              )}
              {jobs.map((job) => {
                const hasReport =
                  !!job.result &&
                  (job.result.errors.length > 0 || job.result.skipped_records.length > 0);
                const isExpanded = expandedJobIds.has(job.id);
                return (
                  <Fragment key={job.id}>
                    <TableRow>
                      <TableCell className="font-mono text-xs">{job.id.slice(0, 12)}</TableCell>
                      <TableCell>
                        <Badge variant={stateBadgeVariant(job.state)}>{job.state}</Badge>
                      </TableCell>
                      <TableCell>
                        <JobResultCell job={job} />
                      </TableCell>
                      <TableCell className="space-x-2">
                        {job.state === 'completed' && job.result && job.result.created > 0 && (
                          <>
                            <Link href={'/books?visibility=pending_review' as any}>
                              <Button variant="outline" size="sm">
                                К очереди на проверку
                              </Button>
                            </Link>
                            <Link href={'/authors/moderation' as any}>
                              <Button variant="outline" size="sm">
                                К авторам на модерации
                              </Button>
                            </Link>
                          </>
                        )}
                        {hasReport && (
                          <Button variant="outline" size="sm" onClick={() => toggleJobExpanded(job.id)}>
                            {isExpanded ? 'Скрыть' : 'Подробнее'}
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                    {isExpanded && hasReport && <JobReportRow job={job} />}
                  </Fragment>
                );
              })}
            </TableBody>
          </Table>
        </div>
      </div>
    </div>
  );
}
