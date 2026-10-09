'use client';

import { useState } from 'react';
import Link from 'next/link';
import { AdminGenreWikidataCandidate } from '@bookhaven/api-client';
import { Button, Card, CardContent } from '@bookhaven/ui';
import {
  GenreImportFeedback,
  genreImportErrorMessage,
  genreImportResultFeedback,
  useImportWikidataGenre,
} from '@/features/import-wikidata-genre';
import { useWikidataGenreSearch, WikidataGenreSearch } from '@/features/search-wikidata-genre';

/**
 * Wikidata genre import, the «Wikidata» section of /import/genres
 * (docs/features/genre-wikidata-import.md, docs/features/import-by-entity.md): the Wikidata
 * literary-genre search with a per-candidate import action and a result
 * banner. After every successful import the last search is re-run so the
 * "already in BookHaven" markers refresh.
 */
export function WikidataGenreImportPanel() {
  const search = useWikidataGenreSearch();
  const { importGenre, pendingQid } = useImportWikidataGenre();
  const [feedback, setFeedback] = useState<GenreImportFeedback | null>(null);

  const run = async (candidate: AdminGenreWikidataCandidate) => {
    setFeedback(null);
    try {
      const result = await importGenre(candidate);
      setFeedback(genreImportResultFeedback(result));
      search.refresh();
    } catch (err) {
      setFeedback({ type: 'error', message: genreImportErrorMessage(err), links: [] });
    }
  };

  const renderActions = (candidate: AdminGenreWikidataCandidate) => {
    const existing = candidate.existing;
    if (existing?.status === 'rejected') {
      return <span className="text-sm text-muted-foreground">Отклонён ранее</span>;
    }
    if (existing) {
      return <span className="text-sm text-muted-foreground">Уже в каталоге</span>;
    }
    if (candidate.ru_label === null && search.classCheck === 'ok') {
      return <span className="text-sm text-muted-foreground">Нет русского названия</span>;
    }
    return (
      <Button
        type="button"
        size="sm"
        disabled={pendingQid === candidate.qid}
        onClick={() => run(candidate)}
      >
        Импортировать
      </Button>
    );
  };

  return (
    <div className="space-y-6">
      {feedback && (
        <div
          role="status"
          className={`flex items-start justify-between gap-4 rounded-md p-3 text-sm ${
            feedback.type === 'success'
              ? 'bg-green-100 text-green-900'
              : 'bg-destructive/10 text-destructive'
          }`}
        >
          <div className="space-y-1">
            <div>{feedback.message}</div>
            {feedback.links.length > 0 && (
              <div className="flex gap-3">
                {feedback.links.map((link) => (
                  <Link key={link.href} href={link.href as any} className="underline">
                    {link.label}
                  </Link>
                ))}
              </div>
            )}
          </div>
          <button type="button" onClick={() => setFeedback(null)} className="underline">
            Скрыть
          </button>
        </div>
      )}

      <Card>
        <CardContent className="p-4">
          <WikidataGenreSearch
            search={search}
            inputId="wikidata-genre-import-search"
            label="Название жанра в Wikidata"
            emptyHint="Литературных жанров не найдено. Проверьте написание или создайте жанр вручную в разделе «Вручную»."
            renderActions={renderActions}
          />
        </CardContent>
      </Card>
    </div>
  );
}
