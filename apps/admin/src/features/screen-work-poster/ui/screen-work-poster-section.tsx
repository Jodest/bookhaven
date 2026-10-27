'use client';

import { useRef, useState, type ChangeEvent } from 'react';
import type { AdminScreenWorkDetail } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { usePosterMutations } from '../model/use-poster-mutations';
import { POSTER_ACCEPT, POSTER_SOURCE_LABELS, posterFileError } from '../lib/poster-text';

interface ScreenWorkPosterSectionProps {
  work: AdminScreenWorkDetail;
}

type Notice = { kind: 'success' | 'error'; text: string };

// «Постер» section of the work edit page (openspec screen-works/admin-editing):
// shows the current poster and its source, uploads a replacement and removes
// it. The actions apply at once and never touch the data form.
export function ScreenWorkPosterSection({ work }: ScreenWorkPosterSectionProps) {
  const { upload, remove, isPending } = usePosterMutations(work.id);
  const [notice, setNotice] = useState<Notice | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const poster = work.poster;

  const handleFile = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    const invalid = posterFileError(file);
    if (invalid) {
      setNotice({ kind: 'error', text: invalid });
      return;
    }
    setNotice(null);
    upload.mutate(file, {
      onSuccess: () => setNotice({ kind: 'success', text: 'Постер загружен.' }),
      onError: () => setNotice({ kind: 'error', text: 'Не удалось загрузить постер.' }),
    });
  };

  const handleRemove = () => {
    if (!confirm(`Удалить постер фильма «${work.title}»?`)) return;
    setNotice(null);
    remove.mutate(undefined, {
      onSuccess: () => setNotice({ kind: 'success', text: 'Постер удалён.' }),
      onError: () => setNotice({ kind: 'error', text: 'Не удалось удалить постер.' }),
    });
  };

  return (
    <section className="space-y-3 rounded-lg border p-4" aria-labelledby="work-poster-title">
      <h2 id="work-poster-title" className="text-lg font-semibold">
        Постер
      </h2>

      {poster ? (
        <div className="flex items-start gap-4">
          <div className="aspect-[2/3] w-32 shrink-0 overflow-hidden rounded-md border bg-muted">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={poster.url} alt={work.title} className="h-full w-full object-cover" />
          </div>
          <div className="space-y-1 text-sm">
            <p>Источник: {POSTER_SOURCE_LABELS[poster.source]}</p>
            {poster.source === 'commons' && (
              <p className="text-muted-foreground">
                Постер: {poster.author ? `${poster.author}, ` : ''}
                {poster.license ? `${poster.license}, ` : ''}
                {poster.source_url ? (
                  <a
                    href={poster.source_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary hover:underline"
                  >
                    Wikimedia Commons
                  </a>
                ) : (
                  'Wikimedia Commons'
                )}
              </p>
            )}
          </div>
        </div>
      ) : (
        <div className="space-y-1 text-sm">
          <p>Постера нет.</p>
          {work.poster_removed && (
            <p className="text-muted-foreground">
              Автоматический постер удалён — синхронизация не подставит его снова.
            </p>
          )}
        </div>
      )}

      <p className="text-xs text-muted-foreground">
        Синхронизация Wikidata подставляет постер только фильмам, которые не правились вручную.
        Для таких фильмов загрузите постер здесь.
      </p>

      <div className="flex flex-wrap gap-2">
        <input
          ref={inputRef}
          type="file"
          accept={POSTER_ACCEPT}
          aria-label="Файл постера"
          className="hidden"
          onChange={handleFile}
        />
        <Button type="button" variant="outline" disabled={isPending} onClick={() => inputRef.current?.click()}>
          {poster ? 'Заменить постер' : 'Загрузить постер'}
        </Button>
        {poster && (
          <Button
            type="button"
            variant="outline"
            className="text-destructive"
            disabled={isPending}
            onClick={handleRemove}
          >
            Удалить постер
          </Button>
        )}
      </div>

      {notice && (
        <div
          role={notice.kind === 'error' ? 'alert' : 'status'}
          className={
            notice.kind === 'error'
              ? 'rounded-md bg-destructive/10 p-3 text-sm text-destructive'
              : 'text-sm text-muted-foreground'
          }
        >
          {notice.text}
        </div>
      )}
    </section>
  );
}
