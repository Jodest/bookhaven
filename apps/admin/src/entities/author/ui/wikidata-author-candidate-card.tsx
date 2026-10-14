'use client';

import { ReactNode } from 'react';
import Link from 'next/link';
import type { AdminHarvestAuthorCandidate } from '@bookhaven/api-client';
import { Card, CardContent } from '@bookhaven/ui';
import { authorModerationHref } from '../lib/moderation-href';
import { AuthorStatusBadge } from './author-status-badge';

interface WikidataAuthorCandidateCardProps {
  candidate: AdminHarvestAuthorCandidate;
  /** Action buttons on the right. */
  actions?: ReactNode;
  /** Optional control on the left (e.g. a selection checkbox). */
  leading?: ReactNode;
}

/**
 * One Wikidata author candidate: label, QID link (new tab), description or
 * «без описания», and the "already in BookHaven" marker with a link. Shared
 * by the Wikidata author search and the AI curation candidate list.
 */
export function WikidataAuthorCandidateCard({ candidate, actions, leading }: WikidataAuthorCandidateCardProps) {
  return (
    <Card>
      <CardContent className="flex items-center justify-between gap-4 p-4">
        <div className="flex items-start gap-3">
          {leading}
          <div className="space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-medium">{candidate.label}</span>
              <a
                href={`https://www.wikidata.org/wiki/${candidate.qid}`}
                target="_blank"
                rel="noopener noreferrer"
                className="font-mono text-xs text-primary underline"
              >
                {candidate.qid}
              </a>
            </div>
            <div className="text-sm text-muted-foreground">{candidate.description || 'без описания'}</div>
            {candidate.existing && (
              <div className="flex flex-wrap items-center gap-2 text-sm">
                <span className="text-muted-foreground">Уже в BookHaven:</span>
                <AuthorStatusBadge status={candidate.existing.status} />
                <Link href={authorModerationHref(candidate.existing) as any} className="text-primary underline">
                  Открыть
                </Link>
              </div>
            )}
          </div>
        </div>
        <div className="flex shrink-0 flex-col items-end gap-1">{actions}</div>
      </CardContent>
    </Card>
  );
}
