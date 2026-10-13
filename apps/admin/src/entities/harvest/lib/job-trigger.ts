import type { AdminHarvestJob } from '@bookhaven/api-client';

/** «Авто» for scheduled jobs, «Вручную» otherwise (manual, CLI, legacy). */
export function harvestJobTriggerLabel(job: Pick<AdminHarvestJob, 'trigger'>): string {
  return job.trigger === 'scheduled' ? 'Авто' : 'Вручную';
}

/** «Авторов: N» when the job's author QIDs are known, else null. */
export function harvestJobAuthorsLabel(job: Pick<AdminHarvestJob, 'author_qids'>): string | null {
  const count = job.author_qids?.length ?? 0;
  return count > 0 ? `Авторов: ${count}` : null;
}
