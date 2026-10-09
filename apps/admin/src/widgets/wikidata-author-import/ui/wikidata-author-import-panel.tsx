'use client';

import { useState } from 'react';
import Link from 'next/link';
import { AdminHarvestAuthorCandidate } from '@bookhaven/api-client';
import { Button, Card, CardContent } from '@bookhaven/ui';
import {
  harvestConfirmText,
  ImportFeedback,
  importErrorMessage,
  importResultFeedback,
  useImportWikidataAuthor,
} from '@/features/import-wikidata-author';
import { useWikidataAuthorSearch, WikidataAuthorSearch } from '@/features/search-wikidata-author';

/**
 * Wikidata author import, the «Wikidata» section of /import/authors
 * (docs/features/author-direct-import.md, docs/features/import-by-entity.md):
 * the shared Wikidata author search with per-candidate import actions and a result banner. After
 * every successful import the last search is re-run so the "already in
 * BookHaven" markers refresh.
 */
export function WikidataAuthorImportPanel() {
  const search = useWikidataAuthorSearch();
  const { importAuthor, importWithBooks, pendingQid } = useImportWikidataAuthor();
  const [feedback, setFeedback] = useState<ImportFeedback | null>(null);

  const run = async (candidate: AdminHarvestAuthorCandidate, withBooks: boolean) => {
    setFeedback(null);
    if (withBooks && !confirm(harvestConfirmText(candidate.existing?.name ?? candidate.label))) {
      return;
    }
    try {
      const outcome = withBooks ? await importWithBooks(candidate) : await importAuthor(candidate);
      setFeedback(importResultFeedback(outcome));
      search.refresh();
    } catch (err) {
      setFeedback({ type: 'error', message: importErrorMessage(err), links: [] });
    }
  };

  const renderActions = (candidate: AdminHarvestAuthorCandidate) => {
    const disabled = pendingQid === candidate.qid;
    const existing = candidate.existing;
    if (existing?.status === 'rejected') {
      return <span className="text-sm text-muted-foreground">Отклонён ранее</span>;
    }
    if (existing) {
      return (
        <Button type="button" size="sm" disabled={disabled} onClick={() => run(candidate, true)}>
          Импортировать книги
        </Button>
      );
    }
    return (
      <>
        <Button type="button" size="sm" disabled={disabled} onClick={() => run(candidate, false)}>
          Импортировать автора
        </Button>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={disabled}
          onClick={() => run(candidate, true)}
        >
          Импортировать с книгами
        </Button>
      </>
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
          <WikidataAuthorSearch
            search={search}
            inputId="wikidata-import-search"
            label="Имя автора в Wikidata"
            emptyHint="Совпадений не найдено. Проверьте написание или создайте автора вручную в разделе «Вручную»."
            renderActions={renderActions}
          />
        </CardContent>
      </Card>
    </div>
  );
}
