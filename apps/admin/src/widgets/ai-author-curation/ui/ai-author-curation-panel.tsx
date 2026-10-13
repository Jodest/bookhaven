'use client';

import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { Card, CardContent } from '@bookhaven/ui';
import { curationStatusLabel } from '@/entities/ai-curation';
import { CurationPromptForm, useCurationRequests, useCurationStatus } from '@/features/request-ai-curation';
import { CurationRequestDetail } from './curation-request-detail';

/**
 * Импорт → Авторы → «Подбор ИИ» (/import/authors/ai, openspec change
 * add-ai-author-curation): the request form, «Недавние подборки» and the
 * selected request (bookmarkable as ?request=<id>). The AI only proposes
 * names; candidates are taken through the existing import and harvest
 * flows and land pending.
 */
export function AiAuthorCurationPanel() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const selectedId = searchParams.get('request');
  const status = useCurationStatus();
  const requests = useCurationRequests();

  const hrefFor = (id: string) => `${pathname}?${new URLSearchParams({ request: id })}`;

  return (
    <div className="space-y-6">
      <Card>
        <CardContent className="p-4">
          {status.isLoading && <p className="text-sm text-muted-foreground">Загрузка...</p>}
          {status.error && (
            <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
              Не удалось загрузить статус подбора ИИ.
            </div>
          )}
          {status.data && (
            <CurationPromptForm status={status.data} onCreated={(id) => router.replace(hrefFor(id) as any)} />
          )}
        </CardContent>
      </Card>

      {selectedId && <CurationRequestDetail key={selectedId} id={selectedId} />}

      <section className="space-y-2">
        <h2 className="text-lg font-semibold">Недавние подборки</h2>
        {requests.isLoading && <p className="text-sm text-muted-foreground">Загрузка...</p>}
        {requests.error && <p className="text-sm text-destructive">Не удалось загрузить подборки.</p>}
        {requests.data && requests.data.items.length === 0 && (
          <p className="text-sm text-muted-foreground">Подборок пока нет.</p>
        )}
        {requests.data && requests.data.items.length > 0 && (
          <ul className="divide-y rounded-md border">
            {requests.data.items.map((r) => (
              <li key={r.id} className={`p-3 text-sm ${r.id === selectedId ? 'bg-accent' : ''}`}>
                <Link href={hrefFor(r.id) as any} className="block space-y-1">
                  <span className="line-clamp-2 font-medium">{r.prompt}</span>
                  <span className="block text-xs text-muted-foreground">
                    {curationStatusLabel(r.status)} · {r.candidates_resolved} из {r.candidates_total} ·{' '}
                    {r.created_by?.username ?? 'удалённый пользователь'} ·{' '}
                    {new Date(r.created_at).toLocaleString('ru-RU')}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
