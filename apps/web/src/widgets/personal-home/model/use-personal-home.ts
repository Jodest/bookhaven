'use client';

import { homeApi, homeKeys } from '@/entities/home';
import { readingChallengesApi, readingChallengeKeys } from '@/entities/reading-challenge';
import { useAuthStore } from '@/entities/session';
import { useQuery } from '@tanstack/react-query';

const STALE_TIME = 60_000;

/**
 * Personal home data for the logged-in user (GET /home/me). The query is
 * disabled for guests and while the session is resolving, so guests never
 * call /home/me. The user ID is part of the key so another account never
 * sees a cached payload.
 */
export function usePersonalHome() {
  const { user, isAuthenticated, isLoading } = useAuthStore();
  const enabled = !isLoading && isAuthenticated;
  const query = useQuery({
    queryKey: homeKeys.me(user?.id),
    queryFn: () => homeApi.me(),
    enabled,
    staleTime: STALE_TIME,
    refetchOnWindowFocus: false,
  });
  return { enabled, query };
}

/**
 * This year's reading challenge (existing GET /reading-challenges/{year}),
 * fetched in parallel with the personal home payload.
 */
export function useCurrentReadingChallenge() {
  const { user, isAuthenticated, isLoading } = useAuthStore();
  const year = new Date().getFullYear();
  const query = useQuery({
    queryKey: readingChallengeKeys.detail(year, user?.id),
    queryFn: () => readingChallengesApi.get(year),
    enabled: !isLoading && isAuthenticated,
    staleTime: STALE_TIME,
    refetchOnWindowFocus: false,
  });
  return { year, query };
}
