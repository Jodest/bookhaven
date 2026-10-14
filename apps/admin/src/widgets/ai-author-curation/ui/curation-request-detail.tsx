'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useQueryClient } from '@tanstack/react-query';
import type { AdminCurationCandidate, AdminCurationOption } from '@bookhaven/api-client';
import { Badge, Button, Card, CardContent } from '@bookhaven/ui';
import {
  curationErrorMessage,
  curationRequestQueryKey,
  curationStatusLabel,
  isCurationActive,
  matchStatusLabel,
} from '@/entities/ai-curation';
import { WikidataAuthorCandidateCard } from '@/entities/author';
import {
  harvestConfirmText,
  ImportFeedback,
  importErrorMessage,
  importResultFeedback,
  useImportWikidataAuthor,
} from '@/features/import-wikidata-author';
import { useCurationRequest } from '@/features/request-ai-curation';
import {
  isOptionSelectable,
  selectAllMatched,
  takeConfirmText,
  takeHarvestLine,
  takeHasProblems,
  takeResultLine,
  useTakeCuratedAuthors,
} from '@/features/take-curated-authors';

const MATCH_BADGE_VARIANT: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  matched: 'default',
  ambiguous: 'secondary',
  not_found: 'outline',
  error: 'destructive',
  pending: 'outline',
};

function FeedbackBanner({ feedback, onClose }: { feedback: ImportFeedback; onClose: () => void }) {
  return (
    <div
      role="status"
      className={`flex items-start justify-between gap-4 rounded-md p-3 text-sm ${
        feedback.type === 'success' ? 'bg-green-100 text-green-900' : 'bg-destructive/10 text-destructive'
      }`}
    >
      <div className="space-y-1">
        <div>{feedback.message}</div>
        {feedback.links.length > 0 && (
          <div className="flex gap-3">
            {feedback.links.map((link) => (
              <Link key={link.href} href={link.href as any} className="underline">
                {link.label}
              </Link>
            ))}
          </div>
        )}
      </div>
      <button type="button" onClick={onClose} className="underline">
        Скрыть
      </button>
    </div>
  );
}

/**
 * One curation request: header, progress «Проверено в Wikidata: X из Y»
 * (polled every 3 s while active), the candidates with their Wikidata
 * options and the take actions. Names and notes are plain text. Nothing is
 * ticked by default.
 */
export function CurationRequestDetail({ id }: { id: string }) {
  const queryClient = useQueryClient();
  const requestKey = curationRequestQueryKey(id);
  const { data: request, isLoading, error } = useCurationRequest(id);
  const { importAuthor, importWithBooks, pendingQid } = useImportWikidataAuthor();
  const take = useTakeCuratedAuthors(requestKey);
  const [selected, setSelected] = useState<string[]>([]);
  const [autoHarvest, setAutoHarvest] = useState(false);
  const [feedback, setFeedback] = useState<ImportFeedback | null>(null);

  if (isLoading) return <p className="text-sm text-muted-foreground">Загрузка подборки...</p>;
  if (error || !request) {
    return (
      <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">Не удалось загрузить подборку.</div>
    );
  }

  const refresh = () => queryClient.invalidateQueries({ queryKey: requestKey });

  const runOne = async (option: AdminCurationOption, withBooks: boolean) => {
    setFeedback(null);
    if (withBooks && !confirm(harvestConfirmText(option.existing?.name ?? option.label))) return;
    try {
      const outcome = withBooks ? await importWithBooks(option) : await importAuthor(option);
      setFeedback(importResultFeedback(outcome));
    } catch (err) {
      setFeedback({ type: 'error', message: importErrorMessage(err), links: [] });
    }
    refresh();
  };

  const toggle = (qid: string) =>
    setSelected((prev) => (prev.includes(qid) ? prev.filter((q) => q !== qid) : [...prev, qid]));

  const allOptions: AdminCurationOption[] = [];
  for (const c of request.candidates) {
    for (const o of c.options) {
      if (!allOptions.some((x) => x.qid === o.qid)) allOptions.push(o);
    }
  }
  const selectedOptions = allOptions.filter((o) => selected.includes(o.qid) && isOptionSelectable(o));

  const runBulk = async (withBooks: boolean) => {
    if (selectedOptions.length === 0) return;
    if (!confirm(takeConfirmText(selectedOptions.length, withBooks, autoHarvest))) return;
    setFeedback(null);
    await take.run({ options: selectedOptions, withBooks, autoHarvest });
    setSelected([]);
  };

  const renderActions = (option: AdminCurationOption) => {
    const disabled = pendingQid === option.qid || take.isRunning;
    const existing = option.existing;
    if (existing?.status === 'rejected') {
      return <span className="text-sm text-muted-foreground">Отклонён ранее</span>;
    }
    if (existing) {
      return (
        <Button type="button" size="sm" disabled={disabled} onClick={() => runOne(option, true)}>
          Импортировать книги
        </Button>
      );
    }
    return (
      <>
        <Button type="button" size="sm" disabled={disabled} onClick={() => runOne(option, false)}>
          Импортировать автора
        </Button>
        <Button type="button" size="sm" variant="outline" disabled={disabled} onClick={() => runOne(option, true)}>
          Импортировать с книгами
        </Button>
      </>
    );
  };

  const renderCandidate = (c: AdminCurationCandidate) => (
    <li key={c.id} className="space-y-2 rounded-md border p-3" data-testid="curation-candidate">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-medium">{c.name}</span>
        <Badge variant={MATCH_BADGE_VARIANT[c.match_status] ?? 'outline'}>{matchStatusLabel(c.match_status)}</Badge>
      </div>
      {c.note && (
        <p className="text-sm text-muted-foreground">
          <span className="italic">Мнение ИИ:</span> {c.note}
        </p>
      )}
      {c.options.length > 0 && (
        <div className="space-y-2">
          {c.options.map((o) => {
            const selectable = isOptionSelectable(o);
            return (
              <WikidataAuthorCandidateCard
                key={o.qid}
                candidate={o}
                actions={renderActions(o)}
                leading={
                  selectable ? (
                    <input
                      type="checkbox"
                      className="mt-1 h-4 w-4"
                      aria-label={`Выбрать ${o.label} (${o.qid})`}
                      checked={selected.includes(o.qid)}
                      disabled={take.isRunning}
                      onChange={() => toggle(o.qid)}
                    />
                  ) : null
                }
              />
            );
          })}
        </div>
      )}
    </li>
  );

  const active = isCurationActive(request.status);
  const summary = take.summary;
  const harvestLine = summary ? takeHarvestLine(summary.harvest) : null;

  return (
    <div className="space-y-4">
      <Card>
        <CardContent className="space-y-2 p-4">
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <Badge variant={request.status === 'failed' ? 'destructive' : 'secondary'}>
              {curationStatusLabel(request.status)}
            </Badge>
            <span className="text-muted-foreground">
              {request.created_by?.username ?? 'удалённый пользователь'} ·{' '}
              {new Date(request.created_at).toLocaleString('ru-RU')} · авторов: до {request.max_names}
            </span>
          </div>
          <p className="whitespace-pre-wrap text-sm">{request.prompt}</p>
          {(active || request.candidates_total > 0) && (
            <p className="text-sm">
              Проверено в Wikidata: {request.candidates_resolved} из {request.candidates_total}
            </p>
          )}
          {active && request.candidates_total === 0 && (
            <p className="text-sm text-muted-foreground">ИИ подбирает авторов…</p>
          )}
          {request.status === 'failed' && request.error_code && (
            <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
              {curationErrorMessage(request.error_code)}
            </div>
          )}
          {request.status === 'done' && request.candidates_total === 0 && (
            <p className="text-sm text-muted-foreground">ИИ не предложил ни одного автора по этому запросу.</p>
          )}
        </CardContent>
      </Card>

      {feedback && <FeedbackBanner feedback={feedback} onClose={() => setFeedback(null)} />}

      {request.candidates.length > 0 && (
        <div className="space-y-3 rounded-md border p-3" aria-label="Массовые действия">
          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={take.isRunning}
              onClick={() => setSelected(selectAllMatched(request.candidates))}
            >
              Выбрать все найденные
            </Button>
            <Button type="button" size="sm" variant="ghost" disabled={take.isRunning} onClick={() => setSelected([])}>
              Снять выбор
            </Button>
            <span className="text-sm text-muted-foreground">Выбрано: {selectedOptions.length}</span>
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              className="h-4 w-4"
              checked={autoHarvest}
              disabled={take.isRunning}
              onChange={(e) => setAutoHarvest(e.target.checked)}
            />
            Включить автопополнение книг
          </label>
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              size="sm"
              disabled={take.isRunning || selectedOptions.length === 0}
              onClick={() => runBulk(false)}
            >
              Импортировать выбранных
            </Button>
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={take.isRunning || selectedOptions.length === 0}
              onClick={() => runBulk(true)}
            >
              Импортировать выбранных с книгами
            </Button>
          </div>
          {take.isRunning && take.progress && (
            <p className="text-sm" aria-live="polite">
              Импорт: {take.progress.done} из {take.progress.total}
            </p>
          )}
          {summary && (
            <div
              role="status"
              className={`space-y-1 rounded-md p-3 text-sm ${
                takeHasProblems(summary) ? 'bg-amber-50 text-amber-900' : 'bg-green-100 text-green-900'
              }`}
            >
              <ul className="list-disc pl-5">
                {summary.results.map((r) => (
                  <li key={r.qid}>{takeResultLine(r)}</li>
                ))}
              </ul>
              {harvestLine && <p>{harvestLine}</p>}
              {summary.harvest.state === 'started' && (
                <Link href="/import/jobs" className="underline">
                  К задачам харвеста
                </Link>
              )}
              <div>
                <button type="button" onClick={take.clearSummary} className="underline">
                  Скрыть
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {request.candidates.length > 0 && <ul className="space-y-3">{request.candidates.map(renderCandidate)}</ul>}
    </div>
  );
}
