'use client';

import { Fragment, useState } from 'react';
import Link from 'next/link';
import {
  harvestJobAuthorsLabel,
  harvestJobStateBadgeVariant,
  harvestJobTriggerLabel,
  useHarvestJobs,
} from '@/entities/harvest';
import { AdminHarvestJob } from '@bookhaven/api-client';
import {
  Badge,
  Button,
  Progress,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@bookhaven/ui';

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
  const enrichment = authorEnrichmentSummary(job);
  if (enrichment) {
    return (
      <div className="space-y-1">
        <div>{jobResultSummary(job)}</div>
        <div className="text-xs text-muted-foreground">{enrichment}</div>
      </div>
    );
  }
  return <>{jobResultSummary(job)}</>;
}

// authorEnrichmentSummary is the end-of-job Wikidata author enrichment
// line (docs/features/author-wikidata-enrichment.md), or null when the
// result carries no counters (older jobs, enrichment not configured).
function authorEnrichmentSummary(job: AdminHarvestJob): string | null {
  const enriched = job.result?.authors_enriched;
  if (job.state !== 'completed' || enriched === undefined) {
    return null;
  }
  const errors = job.result?.author_enrichment_errors ?? 0;
  let text = `Авторов обогащено из Wikidata: ${enriched}`;
  if (errors > 0) {
    text += `, ошибок обогащения: ${errors}`;
  }
  return text;
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
      <TableCell colSpan={5} className="space-y-4 bg-muted/30">
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

/**
 * Harvest jobs page (/import/jobs, docs/features/admin-navigation-restructure.md
 * phase 4): recent asynq harvest jobs, polled every 5 s. Job results feed the
 * book and author moderation queues, hence the header links.
 */
export function HarvestJobsView() {
  const [expandedJobIds, setExpandedJobIds] = useState<Set<string>>(new Set());

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

  const jobsQuery = useHarvestJobs({ refetchInterval: 5000 });

  const jobs = jobsQuery.data?.items ?? [];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold tracking-tight">Задачи харвеста</h1>
        <div className="flex gap-2">
          <Link href={'/moderation/books' as any}>
            <Button variant="outline">К очереди на проверку</Button>
          </Link>
          <Link href={'/moderation/authors' as any}>
            <Button variant="outline">К авторам на модерации</Button>
          </Link>
        </div>
      </div>

      <div className="space-y-2">
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
                <TableHead>Запуск</TableHead>
                <TableHead>Результат</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {jobs.length === 0 && (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground">
                    Нет задач
                  </TableCell>
                </TableRow>
              )}
              {jobs.map((job) => {
                const hasReport =
                  !!job.result &&
                  (job.result.errors.length > 0 || job.result.skipped_records.length > 0);
                const isExpanded = expandedJobIds.has(job.id);
                const authorsLabel = harvestJobAuthorsLabel(job);
                return (
                  <Fragment key={job.id}>
                    <TableRow>
                      <TableCell className="font-mono text-xs">{job.id.slice(0, 12)}</TableCell>
                      <TableCell>
                        <Badge variant={harvestJobStateBadgeVariant(job.state)}>{job.state}</Badge>
                      </TableCell>
                      <TableCell>
                        <div className="space-y-1">
                          <Badge variant={job.trigger === 'scheduled' ? 'default' : 'outline'}>
                            {harvestJobTriggerLabel(job)}
                          </Badge>
                          {authorsLabel && (
                            <div className="text-xs text-muted-foreground">{authorsLabel}</div>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <JobResultCell job={job} />
                      </TableCell>
                      <TableCell className="space-x-2">
                        {job.state === 'completed' && job.result && job.result.created > 0 && (
                          <>
                            <Link href={'/moderation/books' as any}>
                              <Button variant="outline" size="sm">
                                К очереди на проверку
                              </Button>
                            </Link>
                            <Link href={'/moderation/authors' as any}>
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
