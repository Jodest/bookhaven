'use client';

import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Badge, Card, CardContent, CardHeader, CardTitle } from '@bookhaven/ui';
import {
  adminAiDescriptionsApi,
  aiDescriptionErrorLabel,
  aiDescriptionQueryKey,
  aiDescriptionStatusLabel,
  AiDescriptionText,
} from '@/entities/ai-description';
import { GenerateButton, useAiDescriptionStatus } from '@/features/generate-ai-description';
import { ReviewActions } from '@/features/review-ai-description';
import { PANEL_POLL_MAX_MS, PANEL_POLL_MS, shouldPoll, type PanelPoll } from '../lib/poll';

export const OWN_DESCRIPTION_NOTICE = 'У книги есть собственное описание — описание ИИ на сайте не показывается.';

interface Feedback {
  type: 'success' | 'error';
  message: string;
}

interface AiDescriptionPanelProps {
  bookId: string;
  /** The loaded admin book has a non-blank own description. */
  hasOwnDescription: boolean;
  bookStatus: string;
}

/**
 * «Описание ИИ» on the book edit page (openspec catalog-admin/ai-descriptions):
 * the book's AI description record with its status, plain text, model and
 * time, and the generate and review actions. Shown when the book has no
 * own description or has a record. After queueing it polls the record
 * every 4 s until generated_at changes, for at most 3 minutes.
 */
export function AiDescriptionPanel({ bookId, hasOwnDescription, bookStatus }: AiDescriptionPanelProps) {
  const [poll, setPoll] = useState<PanelPoll | null>(null);
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  const status = useAiDescriptionStatus();
  const record = useQuery({
    queryKey: aiDescriptionQueryKey(bookId),
    queryFn: () => adminAiDescriptionsApi.get(bookId),
    refetchInterval: (query) =>
      shouldPoll(poll, query.state.data?.ai_description?.generated_at ?? null, Date.now()) ? PANEL_POLL_MS : false,
  });

  const item = record.data?.ai_description ?? null;
  const generatedAt = item?.generated_at ?? null;

  // Stop polling when the result arrived, or quietly after 3 minutes.
  useEffect(() => {
    if (!poll) return;
    if (generatedAt !== poll.previousGeneratedAt) {
      setPoll(null);
      setFeedback(null);
      return;
    }
    const left = PANEL_POLL_MAX_MS - (Date.now() - poll.startedAt);
    const timer = setTimeout(() => setPoll(null), Math.max(0, left));
    return () => clearTimeout(timer);
  }, [poll, generatedAt]);

  if (record.isLoading || (hasOwnDescription && !item)) return null;

  const eligible = !hasOwnDescription && (bookStatus === 'pending' || bookStatus === 'published');
  const canGenerate = eligible && (status.data?.enabled ?? false);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Описание ИИ</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {hasOwnDescription && <p className="text-sm text-muted-foreground">{OWN_DESCRIPTION_NOTICE}</p>}
        {record.isError && <p className="text-sm text-destructive">Не удалось загрузить описание ИИ.</p>}
        {item ? (
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <Badge variant={item.status === 'approved' ? 'default' : 'secondary'}>
                {aiDescriptionStatusLabel(item.status)}
              </Badge>
              <span>
                {item.model} · {new Date(item.generated_at).toLocaleString('ru-RU')}
                {item.edited ? ' · изменено администратором' : ''}
              </span>
            </div>
            <AiDescriptionText text={item.text} stale={item.stale} />
            {item.status === 'failed' && item.error_code && (
              <p className="text-sm text-muted-foreground">{aiDescriptionErrorLabel(item.error_code)}</p>
            )}
            {item.status === 'unknown' && (
              <p className="text-sm text-muted-foreground">Модель не знает это произведение — описание не создано.</p>
            )}
          </div>
        ) : (
          !poll && <p className="text-sm text-muted-foreground">У книги нет описания. ИИ может предложить черновик.</p>
        )}
        {poll && <p className="text-sm text-muted-foreground">Генерация поставлена в очередь.</p>}
        {feedback && !poll && (
          <div
            role={feedback.type === 'error' ? 'alert' : 'status'}
            className={
              feedback.type === 'error'
                ? 'rounded-md bg-destructive/10 p-3 text-sm text-destructive'
                : 'rounded-md bg-muted p-3 text-sm'
            }
          >
            {feedback.message}
          </div>
        )}
        <div className="flex flex-wrap items-start gap-2">
          {item && !hasOwnDescription && (
            <ReviewActions bookId={bookId} status={item.status} text={item.text} notify={setFeedback} />
          )}
          {canGenerate && (
            <GenerateButton
              bookId={bookId}
              currentStatus={item?.status ?? null}
              notify={(f) => setFeedback(f && f.type === 'error' ? f : null)}
              onQueued={() => setPoll({ previousGeneratedAt: generatedAt, startedAt: Date.now() })}
            />
          )}
        </div>
        <p className="text-xs text-muted-foreground">
          Описание ИИ хранится отдельно от описания книги и показывается на сайте с пометкой «Описание создано
          ИИ и может содержать неточности.» — только после одобрения и только пока у книги нет собственного
          описания.
        </p>
      </CardContent>
    </Card>
  );
}
