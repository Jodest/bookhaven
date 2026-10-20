'use client';

import { useAuthStore } from '@/entities/session';
import { screenWorkKeys, screenWorksApi } from '@/entities/screen-work';
import type { AdaptationVerdicts, AdaptationVoteValue } from '@bookhaven/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

/**
 * Verdicts of every adaptation of the book (one cached query shared by all
 * cards) and the vote/unvote mutations of one adaptation. A vote (value) or a
 * removal (null) is not optimistic: the cache changes only after the server
 * answers, so a failure keeps the previous state.
 */
export function useAdaptationVerdicts(bookId: string, screenWorkId: string) {
  const { user } = useAuthStore();
  const queryClient = useQueryClient();
  const queryKey = screenWorkKeys.verdicts(user?.id, bookId);

  const query = useQuery({
    queryKey,
    queryFn: () => screenWorksApi.verdicts(bookId),
  });

  const mutation = useMutation({
    mutationFn: async (value: AdaptationVoteValue | null) => {
      if (value === null) {
        await screenWorksApi.unvote(bookId, screenWorkId);
        return null;
      }
      return screenWorksApi.vote(bookId, screenWorkId, value);
    },
    onSuccess: (verdict) => {
      if (verdict === null) return queryClient.invalidateQueries({ queryKey });
      queryClient.setQueryData<AdaptationVerdicts>(queryKey, (prev) =>
        prev && {
          ...prev,
          items: prev.items.map((item) =>
            item.screen_work_id === verdict.screen_work_id ? verdict : item,
          ),
        },
      );
    },
    // The vote may have been refused because the shelf state changed.
    onError: () => queryClient.invalidateQueries({ queryKey }),
  });

  return {
    verdicts: query.data,
    isError: query.isError,
    item: query.data?.items.find((i) => i.screen_work_id === screenWorkId),
    setVote: (value: AdaptationVoteValue | null) => mutation.mutate(value),
    isPending: mutation.isPending,
    saveFailed: mutation.isError,
  };
}
