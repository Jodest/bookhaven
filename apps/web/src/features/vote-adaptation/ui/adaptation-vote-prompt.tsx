'use client';

import { useAuthStore } from '@/entities/session';
import { loginHref } from '@/shared/lib/safe-redirect';
import Link from 'next/link';
import { useAdaptationVerdicts } from '../model/use-adaptation-verdicts';

interface AdaptationVotePromptProps {
  bookId: string;
  slug: string;
}

/**
 * One block-level hint under the «Экранизации» heading: a login link for a
 * guest, or the «mark as read» hint for a reader who cannot vote yet. Shares
 * the cached verdicts query with the cards; renders nothing while it loads or
 * after it fails.
 */
export function AdaptationVotePrompt({ bookId, slug }: AdaptationVotePromptProps) {
  const { isAuthenticated } = useAuthStore();
  const { verdicts } = useAdaptationVerdicts(bookId, '');
  if (!verdicts) return null;

  if (!isAuthenticated) {
    return (
      <Link
        href={loginHref(`/books/${slug}`)}
        className="mt-1 block text-sm text-primary hover:underline"
      >
        Войдите, чтобы сравнить книгу и экранизацию
      </Link>
    );
  }
  if (!verdicts.can_vote) {
    return (
      <p className="mt-1 text-sm text-muted-foreground">
        Отметьте книгу как «Прочитано», чтобы сравнить её с экранизацией.
      </p>
    );
  }
  return null;
}
