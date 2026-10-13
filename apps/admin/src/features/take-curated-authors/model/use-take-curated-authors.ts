'use client';

import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { AdminAuthor, AdminCurationOption, ApiError } from '@bookhaven/api-client';
import { adminAuthorsApi, authorToInput } from '@/entities/author';
import { adminHarvestApi, HARVEST_JOBS_QUERY_KEY } from '@/entities/harvest';
import { harvestLimitFor } from '../lib/select';

export type AutoHarvestStep =
  | { state: 'not_requested' }
  | { state: 'already' }
  | { state: 'set' }
  | { state: 'failed'; error: string };

/** Per-author outcome of a bulk take. */
export interface TakeAuthorResult {
  qid: string;
  label: string;
  /** Import outcome; absent when the import call failed. */
  outcome?: 'created' | 'existing' | 'rejected';
  author?: AdminAuthor;
  error?: string;
  autoHarvest: AutoHarvestStep;
}

export type TakeHarvestStep =
  | { state: 'not_requested' }
  | { state: 'skipped' }
  | { state: 'started'; count: number }
  | { state: 'failed'; error: string };

export interface TakeSummary {
  results: TakeAuthorResult[];
  harvest: TakeHarvestStep;
}

export interface TakeOptions {
  /** Ticked options, in display order. */
  options: AdminCurationOption[];
  /** «Импортировать выбранных с книгами»: one harvest job afterwards. */
  withBooks: boolean;
  /** «Включить автопополнение книг». */
  autoHarvest: boolean;
}

function errorText(err: unknown): string {
  if (err instanceof ApiError && err.message) return err.message;
  if (err instanceof Error && err.message) return err.message;
  return 'неизвестная ошибка';
}

/**
 * Takes ticked curation options through the existing flows only, one step
 * at a time (openspec change add-ai-author-curation, Decision 11):
 *
 *  1. POST /admin/authors/import-wikidata for each option (lands pending);
 *  2. with books: one POST /admin/harvest for every created/existing
 *     author's QID, limit min(5000, 300·n);
 *  3. auto-harvest: for each created/existing author not yet flagged,
 *     GET /admin/authors/:id right before PATCH with auto_harvest: true.
 *     PATCH replaces every field, so a concurrent edit in another tab could
 *     be overwritten (accepted for a solo admin).
 *
 * A rejected outcome or an import error never starts a harvest or sets the
 * flag; a failed step is reported per author and does not stop the batch.
 * Nothing is published. Afterwards the authors, the dashboard, the harvest
 * jobs and the given request detail are refreshed.
 */
export function useTakeCuratedAuthors(requestQueryKey?: readonly unknown[]) {
  const queryClient = useQueryClient();
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null);
  const [summary, setSummary] = useState<TakeSummary | null>(null);
  const [isRunning, setIsRunning] = useState(false);

  const run = async ({ options, withBooks, autoHarvest }: TakeOptions): Promise<TakeSummary> => {
    setIsRunning(true);
    setSummary(null);
    setProgress({ done: 0, total: options.length });
    const results: TakeAuthorResult[] = [];
    try {
      for (const option of options) {
        const item: TakeAuthorResult = {
          qid: option.qid,
          label: option.label,
          autoHarvest: { state: 'not_requested' },
        };
        try {
          const res = await adminAuthorsApi.importWikidata(option.qid);
          item.outcome = res.outcome;
          item.author = res.author;
        } catch (err) {
          item.error = errorText(err);
        }
        results.push(item);
        setProgress({ done: results.length, total: options.length });
      }

      const taken = results.filter(
        (r) => r.author && (r.outcome === 'created' || r.outcome === 'existing')
      );

      let harvest: TakeHarvestStep = { state: 'not_requested' };
      if (withBooks) {
        const qids: string[] = [];
        for (const r of taken) {
          const qid = r.author?.external_id || r.qid;
          if (!qids.includes(qid)) qids.push(qid);
        }
        if (qids.length === 0) {
          harvest = { state: 'skipped' };
        } else {
          try {
            await adminHarvestApi.trigger({ author_qids: qids, limit: harvestLimitFor(qids.length) });
            harvest = { state: 'started', count: qids.length };
          } catch (err) {
            harvest = { state: 'failed', error: errorText(err) };
          }
        }
      }

      if (autoHarvest) {
        for (const r of taken) {
          const author = r.author as AdminAuthor;
          if (author.auto_harvest) {
            r.autoHarvest = { state: 'already' };
            continue;
          }
          try {
            const fresh = await adminAuthorsApi.get(author.id);
            if (fresh.auto_harvest) {
              r.autoHarvest = { state: 'already' };
              continue;
            }
            await adminAuthorsApi.update(author.id, { ...authorToInput(fresh), auto_harvest: true });
            r.autoHarvest = { state: 'set' };
          } catch (err) {
            r.autoHarvest = { state: 'failed', error: errorText(err) };
          }
        }
      }

      const out: TakeSummary = { results, harvest };
      setSummary(out);
      return out;
    } finally {
      setIsRunning(false);
      queryClient.invalidateQueries({ queryKey: ['admin', 'authors'] });
      queryClient.invalidateQueries({ queryKey: ['admin', 'dashboard'] });
      queryClient.invalidateQueries({ queryKey: HARVEST_JOBS_QUERY_KEY });
      if (requestQueryKey) {
        queryClient.invalidateQueries({ queryKey: requestQueryKey });
      }
    }
  };

  return {
    run,
    isRunning,
    progress,
    summary,
    clearSummary: () => setSummary(null),
  };
}
