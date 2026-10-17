'use client';

import { Button } from '@bookhaven/ui';
import { ANALYZE_CONFIRM_TEXT, ANALYZE_QUEUED_TEXT, analyzeErrorMessage } from '../lib/messages';
import { useAnalyzeGenres } from '../model/use-suggestion-mutations';
import type { SuggestionFeedback } from '../model/types';

interface AnalyzeButtonProps {
  // True while a run is queued or running: starting another one is refused
  // by the server (409), so the button waits.
  running?: boolean;
  notify: (feedback: SuggestionFeedback | null) => void;
}

/**
 * «Запустить анализ»: asks for confirmation, then queues a manual AI genre
 * analysis. The page shows this button only while the analysis is switched
 * on. Nothing changes in the genres; suggestions appear when the worker is
 * done.
 */
export function AnalyzeButton({ running = false, notify }: AnalyzeButtonProps) {
  const mutation = useAnalyzeGenres();

  const start = () => {
    notify(null);
    if (!window.confirm(ANALYZE_CONFIRM_TEXT)) return;
    mutation.mutate(undefined, {
      onSuccess: () => notify({ type: 'success', message: ANALYZE_QUEUED_TEXT }),
      onError: (err) => notify({ type: 'error', message: analyzeErrorMessage(err) }),
    });
  };

  return (
    <Button size="sm" variant="outline" disabled={mutation.isPending || running} onClick={start}>
      Запустить анализ
    </Button>
  );
}
