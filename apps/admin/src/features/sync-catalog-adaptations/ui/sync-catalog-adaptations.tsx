'use client';

import { useState } from 'react';
import { ApiError } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { useTriggerCatalogAdaptationSync } from '@/entities/harvest';

const CONFIRM_TEXT =
  'Найти экранизации в Wikidata для всех опубликованных книг с QID? Задача выполнится в фоне.';
const QUEUED_TEXT = 'Поиск экранизаций по каталогу поставлен в очередь.';
const RUNNING_TEXT = 'Поиск экранизаций по каталогу уже выполняется.';
const FAILED_TEXT = 'Не удалось запустить поиск экранизаций.';

interface SyncCatalogAdaptationsProps {
  // True while the job list holds a catalog sync job that is pending, active
  // or retrying: the server refuses another one (409), so the button waits.
  running?: boolean;
}

type Feedback = { type: 'success' | 'error'; message: string } | null;

/**
 * «Найти экранизации для всего каталога»: asks for confirmation, then queues
 * the catalog-wide adaptation sync job. Links are only added; hidden links
 * stay hidden and hand-edited works are untouched.
 */
export function SyncCatalogAdaptations({ running = false }: SyncCatalogAdaptationsProps) {
  const mutation = useTriggerCatalogAdaptationSync();
  const [confirming, setConfirming] = useState(false);
  const [feedback, setFeedback] = useState<Feedback>(null);

  const start = () => {
    setConfirming(false);
    mutation.mutate(undefined, {
      onSuccess: () => setFeedback({ type: 'success', message: QUEUED_TEXT }),
      onError: (err) =>
        setFeedback({
          type: 'error',
          message: err instanceof ApiError && err.status === 409 ? RUNNING_TEXT : FAILED_TEXT,
        }),
    });
  };

  return (
    <div className="flex flex-col items-end gap-2">
      <Button
        variant="outline"
        disabled={mutation.isPending || running}
        onClick={() => {
          setFeedback(null);
          setConfirming(true);
        }}
      >
        Найти экранизации для всего каталога
      </Button>

      {confirming && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-black/50" onClick={() => setConfirming(false)} />
          <div
            role="dialog"
            aria-modal="true"
            aria-label="Поиск экранизаций по каталогу"
            className="relative z-10 w-full max-w-md space-y-4 rounded-lg border bg-background p-6 shadow-lg"
          >
            <p className="text-sm">{CONFIRM_TEXT}</p>
            <div className="flex justify-end gap-2">
              <Button type="button" variant="outline" onClick={() => setConfirming(false)}>
                Отмена
              </Button>
              <Button type="button" onClick={start}>
                Запустить
              </Button>
            </div>
          </div>
        </div>
      )}

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
    </div>
  );
}
