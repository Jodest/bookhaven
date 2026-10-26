'use client';

import { ADAPTATION_VOTE_LABELS, ADAPTATION_VOTE_SHORT_LABELS, verdictLabel } from '@/entities/screen-work';
import { useAuthStore } from '@/entities/session';
import type { AdaptationVoteValue } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { useAdaptationVerdicts } from '../model/use-adaptation-verdicts';

interface AdaptationVerdictProps {
  bookId: string;
  screenWorkId: string;
}

const VOTE_VALUES: AdaptationVoteValue[] = ['book', 'adaptation', 'same'];

/**
 * Community verdict of one adaptation and the voting controls under it.
 * Renders nothing until the verdicts load, and when they fail or the
 * adaptation has no item, so the card stays usable.
 */
export function AdaptationVerdict({ bookId, screenWorkId }: AdaptationVerdictProps) {
  const { isAuthenticated } = useAuthStore();
  const { verdicts, item, setVote, isPending, saveFailed } = useAdaptationVerdicts(
    bookId,
    screenWorkId,
  );
  if (!verdicts || !item) return null;

  const myVote = item.my_vote;

  return (
    <div className="mt-3 space-y-2 border-t pt-3 text-sm">
      <p className="font-medium">{verdictLabel(item, verdicts.min_votes)}</p>

      {isAuthenticated && verdicts.can_vote ? (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <span className="text-muted-foreground">Что лучше:</span>
          <div role="group" aria-label="Что лучше" className="inline-flex">
            {VOTE_VALUES.map((value) => (
              <Button
                key={value}
                size="sm"
                variant={myVote === value ? 'default' : 'outline'}
                aria-pressed={myVote === value}
                disabled={isPending}
                onClick={() => setVote(myVote === value ? null : value)}
                className="rounded-none first:rounded-l-md last:rounded-r-md [&:not(:first-child)]:-ml-px"
              >
                {ADAPTATION_VOTE_SHORT_LABELS[value]}
              </Button>
            ))}
          </div>
        </div>
      ) : isAuthenticated && myVote ? (
        <div className="flex flex-wrap items-center gap-2">
          <span>Ваш голос: {ADAPTATION_VOTE_LABELS[myVote]}</span>
          <Button size="sm" variant="outline" disabled={isPending} onClick={() => setVote(null)}>
            Убрать голос
          </Button>
        </div>
      ) : null}

      {saveFailed && (
        <p role="alert" className="text-destructive">
          Не удалось сохранить голос.
        </p>
      )}
    </div>
  );
}
