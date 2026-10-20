import type { AdminAdaptationSyncErrorReason, AdminHarvestJob } from '@bookhaven/api-client';
import { isHarvestJobInFlight } from './job-state';

/** Whether the job is a catalog-wide adaptation sync (not a harvest). */
export function isAdaptationSyncJob(job: Pick<AdminHarvestJob, 'kind'>): boolean {
  return job.kind === 'adaptation_sync';
}

/** Completed summary: «Книг: 683 из 683, обновлено: 40, добавлено экранизаций: 75, ошибок: 0». */
export function adaptationSyncSummary(job: Pick<AdminHarvestJob, 'adaptation_sync' | 'last_error'>): string {
  const r = job.adaptation_sync;
  if (!r) return job.last_error || '—';
  return `Книг: ${r.processed} из ${r.total}, обновлено: ${r.updated}, добавлено экранизаций: ${r.added}, ошибок: ${r.failed}`;
}

/** Progress of an active catalog sync, or null while the total is unknown. */
export function adaptationSyncProgress(
  job: Pick<AdminHarvestJob, 'adaptation_sync'>
): { processed: number; total: number; percent: number } | null {
  const r = job.adaptation_sync;
  if (!r || r.total === 0) return null;
  const processed = Math.min(r.processed, r.total);
  return { processed, total: r.total, percent: Math.round((processed / r.total) * 100) };
}

export function adaptationSyncReasonLabel(reason: AdminAdaptationSyncErrorReason | string): string {
  switch (reason) {
    case 'wikidata_unavailable':
      return 'Wikidata недоступна';
    case 'save_failed':
      return 'Не удалось сохранить экранизации';
    default:
      return reason;
  }
}

/** «Ещё ошибок: N» when `failed` exceeds the listed errors, else null. */
export function adaptationSyncMoreErrors(job: Pick<AdminHarvestJob, 'adaptation_sync'>): string | null {
  const r = job.adaptation_sync;
  if (!r) return null;
  const more = r.failed - r.errors.length;
  return more > 0 ? `Ещё ошибок: ${more}` : null;
}

/** Whether the list holds a catalog sync job that is pending, active or retrying. */
export function hasAdaptationSyncInFlight(jobs?: Pick<AdminHarvestJob, 'kind' | 'state'>[]): boolean {
  return !!jobs && jobs.some((j) => isAdaptationSyncJob(j) && isHarvestJobInFlight(j));
}
