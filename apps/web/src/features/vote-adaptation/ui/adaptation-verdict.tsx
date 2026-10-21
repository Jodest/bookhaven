'use client';

import { ADAPTATION_VOTE_LABELS, verdictLabel } from '@/entities/screen-work';
import { useAuthStore } from '@/entities/session';
import { loginHref } from '@/shared/lib/safe-redirect';
import type { AdaptationVoteValue } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { useAdaptationVerdicts } from '../model/use-adaptation-verdicts';

interface AdaptationVerdictProps {
  bookId: string;
  slug: string;
  screenWorkId: string;
}

const VOTE_VALUES: AdaptationVoteValue[] = ['book', 'adaptation', 'same'];

/**
 * Community verdict of one adaptation and the voting controls under it.
 * Renders nothing until the verdicts load, and when they fail or the
 * adaptation has no item, so the card stays usable.
 */
export function AdaptationVerdict({ bookId, slug, screenWorkId }: AdaptationVerdictProps) {
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

      {!isAuthenticated ? (
        <Link href={loginHref(`/books/${slug}`)} className="block text-primary hover:underline">
          Войдите, чтобы сравнить книгу и экранизацию
        </Link>
      ) : verdicts.can_vote ? (
        <div className="space-y-2">
          <p className="text-muted-foreground">Что лучше?</p>
          <div className="flex flex-wrap gap-2">
            {VOTE_VALUES.map((value) => (
              <Button
                key={value}
                size="sm"
                variant={myVote === value ? 'default' : 'outline'}
                aria-pressed={myVote === value}
                disabled={isPending}
                onClick={() => setVote(myVote === value ? null : value)}
              >
                {ADAPTATION_VOTE_LABELS[value]}
              </Button>
            ))}
          </div>
          <p className="text-xs text-muted-foreground">
            Голосуя, вы отмечаете, что смотрели экранизацию.
          </p>
        </div>
      ) : myVote ? (
        <div className="flex flex-wrap items-center gap-2">
          <span>Ваш голос: {ADAPTATION_VOTE_LABELS[myVote]}</span>
          <Button size="sm" variant="outline" disabled={isPending} onClick={() => setVote(null)}>
            Убрать голос
          </Button>
        </div>
      ) : (
        <p className="text-muted-foreground">
          Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.
        </p>
      )}

      {saveFailed && (
        <p role="alert" className="text-destructive">
          Не удалось сохранить голос.
        </p>
      )}
    </div>
  );
}
