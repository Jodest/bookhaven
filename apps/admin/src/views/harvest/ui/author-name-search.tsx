'use client';

import { Badge, Button, Label } from '@bookhaven/ui';
import { useWikidataAuthorSearch, WikidataAuthorSearch } from '@/features/search-wikidata-author';

export interface SelectedAuthor {
  qid: string;
  label: string;
}

interface AuthorNameSearchProps {
  selectedAuthors: SelectedAuthor[];
  onAdd: (author: SelectedAuthor) => void;
  onRemove: (qid: string) => void;
}

// AuthorNameSearch is the default mode of the harvest form's author-input
// section (see docs/features/harvest-author-search.md): the shared Wikidata
// author search widget (features/search-wikidata-author) with a per-candidate
// "Добавить" action the admin must explicitly click before the QID is added
// to the accumulated selectedAuthors list the parent view submits as
// author_qids. There is deliberately no auto-pick-first-match, even for a
// single result.
export function AuthorNameSearch({ selectedAuthors, onAdd, onRemove }: AuthorNameSearchProps) {
  const search = useWikidataAuthorSearch();
  const selectedQids = new Set(selectedAuthors.map((a) => a.qid));

  return (
    <div className="space-y-4">
      <WikidataAuthorSearch
        search={search}
        emptyHint="Совпадений не найдено. Проверьте написание или используйте ручной ввод QID."
        renderActions={(candidate) => {
          const alreadyAdded = selectedQids.has(candidate.qid);
          return (
            <Button
              type="button"
              variant={alreadyAdded ? 'outline' : 'default'}
              size="sm"
              disabled={alreadyAdded}
              onClick={() => onAdd({ qid: candidate.qid, label: candidate.label })}
            >
              {alreadyAdded ? 'Добавлено' : 'Добавить'}
            </Button>
          );
        }}
      />

      {selectedAuthors.length > 0 && (
        <div className="space-y-2">
          <Label>Выбранные авторы</Label>
          <div className="flex flex-wrap gap-2">
            {selectedAuthors.map((author) => (
              <Badge
                key={author.qid}
                variant="secondary"
                className="flex items-center gap-2 py-1 pl-3 pr-2"
              >
                {author.label}
                <button
                  type="button"
                  aria-label={`Удалить ${author.label}`}
                  onClick={() => onRemove(author.qid)}
                  className="text-muted-foreground hover:text-foreground"
                >
                  ×
                </button>
              </Badge>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
