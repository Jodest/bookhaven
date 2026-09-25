'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import { adminHarvestApi } from '@/entities/harvest';
import { ApiError, AdminHarvestJob } from '@bookhaven/api-client';
import {
  Badge,
  Button,
  Input,
  Label,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
} from '@bookhaven/ui';

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

export function HarvestView() {
  const [qidsText, setQidsText] = useState('');
  const [limitText, setLimitText] = useState('');
  const [error, setError] = useState<string | null>(null);
  const queryClient = useQueryClient();

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
    const authorQids = parseAuthorQIDs(qidsText);
    if (authorQids.length === 0) {
      setError('Введите хотя бы один QID автора.');
      return;
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
        <Link href={'/books?visibility=pending_review' as any}>
          <Button variant="outline">К очереди на проверку</Button>
        </Link>
      </div>

      <form onSubmit={handleSubmit} className="max-w-xl space-y-4 rounded-lg border p-6">
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
              {jobs.map((job) => (
                <TableRow key={job.id}>
                  <TableCell className="font-mono text-xs">{job.id.slice(0, 12)}</TableCell>
                  <TableCell>
                    <Badge variant={stateBadgeVariant(job.state)}>{job.state}</Badge>
                  </TableCell>
                  <TableCell>{jobResultSummary(job)}</TableCell>
                  <TableCell>
                    {job.state === 'completed' && job.result && job.result.created > 0 && (
                      <Link href={'/books?visibility=pending_review' as any}>
                        <Button variant="outline" size="sm">
                          К очереди на проверку
                        </Button>
                      </Link>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </div>
    </div>
  );
}
