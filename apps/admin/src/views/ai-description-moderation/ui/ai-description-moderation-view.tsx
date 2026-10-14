'use client';

import { useState } from 'react';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import type { AdminAIDescriptionItem, AIDescriptionStatus } from '@bookhaven/api-client';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@bookhaven/ui';
import {
  adminAiDescriptionsApi,
  AI_DESCRIPTION_STATUSES,
  aiDescriptionErrorLabel,
  aiDescriptionsListQueryKey,
  AiDescriptionText,
  isAIDescriptionStatus,
  STATUS_FILTER_LABELS,
} from '@/entities/ai-description';
import { GenerateMissingButton, useAiDescriptionStatus } from '@/features/generate-ai-description';
import { ReviewActions } from '@/features/review-ai-description';
import { buildEditHref } from '@/shared/lib';
import { Pagination } from '@/shared/ui';

// «Описания ИИ» review tab (openspec catalog-admin/ai-descriptions): one AI
// description per row, reviewed one at a time. Deliberately not the shared
// moderation-queue widget: no selection and no bulk approval.

const LIMIT = 20;

interface Feedback {
  type: 'success' | 'error';
  message: string;
}

export function AiDescriptionModerationView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const rawStatus = searchParams.get('status');
  const status: AIDescriptionStatus = isAIDescriptionStatus(rawStatus) ? rawStatus : 'draft';
  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10) || 1);
  const [feedback, setFeedback] = useState<Feedback | null>(null);

  const aiStatus = useAiDescriptionStatus();
  const params = { status, page, limit: LIMIT };
  const list = useQuery({
    queryKey: aiDescriptionsListQueryKey(params),
    queryFn: () => adminAiDescriptionsApi.list(params),
  });

  const pushParams = (updates: Record<string, string | undefined>) => {
    const next = new URLSearchParams(searchParams.toString());
    Object.entries(updates).forEach(([key, value]) => {
      if (value === undefined || value === '') next.delete(key);
      else next.set(key, value);
    });
    setFeedback(null);
    router.push(`${pathname}?${next.toString()}` as any);
  };

  const editHref = (id: string) => buildEditHref(`/catalog/books/${id}/edit`, pathname, searchParams);
  const totalPages = list.data ? Math.ceil(list.data.total / LIMIT) : 0;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-3xl font-bold tracking-tight">Описания ИИ</h1>
        {aiStatus.data?.enabled && (
          <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Генерация описаний ИИ">
            <span className="text-sm text-muted-foreground">Книг без описания: {aiStatus.data.missing}</span>
            <GenerateMissingButton
              missing={aiStatus.data.missing}
              maxBulk={aiStatus.data.max_bulk}
              notify={setFeedback}
            />
          </div>
        )}
      </div>

      <div className="flex flex-wrap gap-2" role="group" aria-label="Статус описания">
        {AI_DESCRIPTION_STATUSES.map((s) => (
          <button
            key={s}
            type="button"
            aria-pressed={s === status}
            onClick={() => pushParams({ status: s === 'draft' ? undefined : s, page: undefined })}
            className={
              s === status
                ? 'rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground'
                : 'rounded-md border px-3 py-1.5 text-sm hover:bg-accent'
            }
          >
            {STATUS_FILTER_LABELS[s]}
          </button>
        ))}
      </div>

      {feedback && (
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

      {list.isLoading ? (
        <div className="h-32 animate-pulse rounded-lg bg-muted" data-testid="list-loading" />
      ) : list.isError ? (
        <div className="rounded-md bg-destructive/10 p-6 text-destructive">Не удалось загрузить описания ИИ.</div>
      ) : list.data && list.data.items.length === 0 ? (
        <p className="text-sm text-muted-foreground">Описаний нет.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Книга</TableHead>
              <TableHead>Описание</TableHead>
              <TableHead>Действия</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {list.data?.items.map((item) => (
              <DescriptionRow key={item.book.id} item={item} editHref={editHref} notify={setFeedback} />
            ))}
          </TableBody>
        </Table>
      )}

      <Pagination page={page} totalPages={totalPages} onPageChange={(p) => pushParams({ page: String(p) })} />
    </div>
  );
}

interface DescriptionRowProps {
  item: AdminAIDescriptionItem;
  editHref: (id: string) => string;
  notify: (feedback: Feedback | null) => void;
}

function DescriptionRow({ item, editHref, notify }: DescriptionRowProps) {
  const meta = [item.book.authors.join(', '), item.book.published_year != null ? String(item.book.published_year) : '']
    .filter(Boolean)
    .join(' · ');
  return (
    <TableRow>
      <TableCell className="align-top">
        <Link href={editHref(item.book.id) as any} className="font-medium hover:underline">
          {item.book.title}
        </Link>
        {meta && <div className="text-xs text-muted-foreground">{meta}</div>}
        <div className="text-xs text-muted-foreground">
          {item.model} · {new Date(item.generated_at).toLocaleString('ru-RU')}
          {item.edited ? ' · изменено' : ''}
        </div>
      </TableCell>
      <TableCell className="max-w-xl align-top">
        <AiDescriptionText text={item.text} stale={item.stale} superseded={item.superseded} />
        {item.status === 'failed' && item.error_code && (
          <p className="text-sm text-muted-foreground">{aiDescriptionErrorLabel(item.error_code)}</p>
        )}
      </TableCell>
      <TableCell className="align-top">
        {!item.superseded && (
          <ReviewActions bookId={item.book.id} status={item.status} text={item.text} notify={notify} />
        )}
      </TableCell>
    </TableRow>
  );
}
