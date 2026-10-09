'use client';

import { KeyboardEvent, ReactNode } from 'react';
import Link from 'next/link';
import { AdminGenreWikidataCandidate } from '@bookhaven/api-client';
import { Button, Card, CardContent, Input, Label } from '@bookhaven/ui';
import { GenreStatusBadge, genreModerationHref } from '@/entities/genre';
import type { WikidataGenreSearch as SearchState } from '../model/use-wikidata-genre-search';

interface WikidataGenreSearchProps {
  /** Controller from useWikidataGenreSearch (owned by the parent so it can refresh). */
  search: SearchState;
  /** Action buttons rendered on the right of each candidate card. */
  renderActions: (candidate: AdminGenreWikidataCandidate) => ReactNode;
  /** Text shown when a search returned zero candidates. */
  emptyHint: string;
  inputId?: string;
  label?: string;
}

/**
 * Wikidata literary-genre search by name (docs/features/genre-wikidata-import.md):
 * input + "Найти" (Enter searches), error/empty states, an "unchecked" warning
 * and candidate cards (Russian name, description, QID link, "already in
 * BookHaven" marker). Actions per candidate are supplied by the parent.
 */
export function WikidataGenreSearch({
  search,
  renderActions,
  emptyHint,
  inputId = 'genre-name-search',
  label = 'Название жанра',
}: WikidataGenreSearchProps) {
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
            placeholder="фэнтези"
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

      {search.classCheck === 'unavailable' && search.candidates.length > 0 && (
        <div role="alert" className="rounded-md bg-amber-100 p-3 text-sm text-amber-900">
          Не удалось проверить, что это литературные жанры — проверка будет выполнена при импорте.
        </div>
      )}

      {search.candidates.length > 0 && (
        <div className="space-y-2">
          {search.candidates.map((candidate) => (
            <Card key={candidate.qid}>
              <CardContent className="flex items-center justify-between gap-4 p-4">
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{candidate.ru_label ?? candidate.label}</span>
                    {candidate.ru_label === null && (
                      <span className="text-xs text-muted-foreground">нет русского названия</span>
                    )}
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
                      <GenreStatusBadge status={candidate.existing.status} />
                      {candidate.existing.via_alias && (
                        <span className="text-muted-foreground">
                          через слитый жанр «{candidate.existing.via_alias.name}»
                        </span>
                      )}
                      <Link
                        href={genreModerationHref(candidate.existing) as any}
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
