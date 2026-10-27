'use client';

import { ADAPTATION_VOTE_LABELS, ADAPTATION_VOTE_SHORT_LABELS, verdictLabel } from '@/entities/screen-work';
import { useAuthStore } from '@/entities/session';
import type { AdaptationVoteValue } from '@bookhaven/api-client';
import { Button, Select } from '@bookhaven/ui';
import { useId } from 'react';
import { useAdaptationVerdicts } from '../model/use-adaptation-verdicts';

interface AdaptationVerdictProps {
  bookId: string;
  screenWorkId: string;
}

const VOTE_VALUES: AdaptationVoteValue[] = ['book', 'adaptation', 'same'];
const REMOVE_VALUE = 'remove';

/**
 * Community verdict of one adaptation and a compact «Что лучше:» dropdown
 * under it. Renders nothing until the verdicts load, and when they fail or
 * the adaptation has no item, so the card stays usable. With no votes there
 * is no verdict line.
 */
export function AdaptationVerdict({ bookId, screenWorkId }: AdaptationVerdictProps) {
  const selectId = useId();
  const { isAuthenticated } = useAuthStore();
  const { verdicts, item, setVote, isPending, saveFailed } = useAdaptationVerdicts(
    bookId,
    screenWorkId,
  );
  if (!verdicts || !item) return null;

  const myVote = item.my_vote;
  const label = verdictLabel(item, verdicts.min_votes);
  const canChoose = isAuthenticated && verdicts.can_vote;
  const showOwnVote = !canChoose && isAuthenticated && myVote !== null;
  if (!label && !canChoose && !showOwnVote && !saveFailed) return null;

  const options = [
    ...(myVote ? [] : [{ value: '', label: 'Не выбрано' }]),
    ...VOTE_VALUES.map((value) => ({ value, label: ADAPTATION_VOTE_SHORT_LABELS[value] })),
    ...(myVote ? [{ value: REMOVE_VALUE, label: 'Убрать голос' }] : []),
  ];

  return (
    <div className="mt-2 space-y-2 text-sm">
      {label && <p className="font-medium">{label}</p>}

      {canChoose ? (
        <div className="flex flex-wrap items-center gap-2">
          <label htmlFor={selectId} className="text-muted-foreground">
            Что лучше:
          </label>
          <Select
            id={selectId}
            className="h-8 w-auto py-0"
            options={options}
            value={myVote ?? ''}
            disabled={isPending}
            onChange={(e) => {
              const value = e.target.value;
              if (value === REMOVE_VALUE) setVote(null);
              else if (value !== '') setVote(value as AdaptationVoteValue);
            }}
          />
        </div>
      ) : showOwnVote && myVote ? (
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
