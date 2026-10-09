'use client';

import { KeyboardEvent, ReactNode } from 'react';
import Link from 'next/link';
import { AdminHarvestAuthorCandidate } from '@bookhaven/api-client';
import { Button, Card, CardContent, Input, Label } from '@bookhaven/ui';
import { AuthorStatusBadge, authorModerationHref } from '@/entities/author';
import type { WikidataAuthorSearch as SearchState } from '../model/use-wikidata-author-search';

interface WikidataAuthorSearchProps {
  /** Controller from useWikidataAuthorSearch (owned by the parent so it can refresh). */
  search: SearchState;
  /** Action buttons rendered on the right of each candidate card. */
  renderActions: (candidate: AdminHarvestAuthorCandidate) => ReactNode;
  /** Hint shown when a search returned zero candidates (text or inline markup, e.g. a link). */
  emptyHint: ReactNode;
  inputId?: string;
  label?: string;
}

/**
 * Wikidata author search by name (see docs/features/harvest-author-search.md
 * and docs/features/author-direct-import.md): input + "Найти", error/empty
 * states and candidate cards (label, description, QID link, "already in
 * BookHaven" marker). Actions per candidate are supplied by the parent.
 *
 * No <form> here: on /import/books/wikidata the widget is nested in the harvest form, and
 * nested forms are invalid HTML - Enter-to-search is wired via onKeyDown.
 */
export function WikidataAuthorSearch({
  search,
  renderActions,
  emptyHint,
  inputId = 'author-name-search',
  label = 'Поиск автора по имени',
}: WikidataAuthorSearchProps) {
  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      search.search();
    }
  };

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor={inputId}>{label}</Label>
        <div className="flex gap-2">
          <Input
            id={inputId}
            value={search.name}
            onChange={(e) => search.setName(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Толстой"
          />
          <Button type="button" onClick={search.search} disabled={search.isPending}>
            {search.isPending ? 'Поиск...' : 'Найти'}
          </Button>
        </div>
      </div>

      {search.error && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {search.error}
        </div>
      )}

      {!search.error && search.hasSearched && search.candidates.length === 0 && (
        <p className="text-sm text-muted-foreground">{emptyHint}</p>
      )}

      {search.candidates.length > 0 && (
        <div className="space-y-2">
          {search.candidates.map((candidate) => (
            <Card key={candidate.qid}>
              <CardContent className="flex items-center justify-between gap-4 p-4">
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
                  <div className="text-sm text-muted-foreground">
                    {candidate.description || 'без описания'}
                  </div>
                  {candidate.existing && (
                    <div className="flex flex-wrap items-center gap-2 text-sm">
                      <span className="text-muted-foreground">Уже в BookHaven:</span>
                      <AuthorStatusBadge status={candidate.existing.status} />
                      <Link
                        href={authorModerationHref(candidate.existing) as any}
                        className="text-primary underline"
                      >
                        Открыть
                      </Link>
                    </div>
                  )}
                </div>
                <div className="flex shrink-0 flex-col items-end gap-1">{renderActions(candidate)}</div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
