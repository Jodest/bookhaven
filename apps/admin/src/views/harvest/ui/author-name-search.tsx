'use client';

import { KeyboardEvent, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { adminHarvestApi } from '@/entities/harvest';
import { ApiError, AdminHarvestAuthorCandidate } from '@bookhaven/api-client';
import { Badge, Button, Card, CardContent, Input, Label } from '@bookhaven/ui';

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
// section (see docs/features/harvest-author-search.md): a name search
// against GET /admin/harvest/authors/search, rendering candidates the admin
// must explicitly confirm ("Добавить") before their QID is added to the
// accumulated selectedAuthors list the parent view submits as author_qids.
// There is deliberately no auto-pick-first-match, even for a single result.
export function AuthorNameSearch({ selectedAuthors, onAdd, onRemove }: AuthorNameSearchProps) {
  const [name, setName] = useState('');
  const [searchError, setSearchError] = useState<string | null>(null);
  const [candidates, setCandidates] = useState<AdminHarvestAuthorCandidate[]>([]);
  const [hasSearched, setHasSearched] = useState(false);

  const searchMutation = useMutation({
    mutationFn: adminHarvestApi.searchAuthors,
    onSuccess: (data) => {
      setSearchError(null);
      setCandidates(data.items);
      setHasSearched(true);
    },
    onError: (err) => {
      let message = 'Не удалось выполнить поиск. Попробуйте снова.';
      if (err instanceof ApiError && err.message) {
        message = err.message;
      }
      setSearchError(message);
      setCandidates([]);
      setHasSearched(true);
    },
  });

  const handleSearch = () => {
    const trimmed = name.trim();
    if (!trimmed) return;
    searchMutation.mutate(trimmed);
  };

  // No <form> here (this component is itself nested inside harvest-view's
  // outer <form>, and nested <form> elements are invalid HTML / cause
  // unpredictable submit bubbling) - Enter-to-search is wired directly via
  // onKeyDown instead of relying on native form submit.
  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleSearch();
    }
  };

  const selectedQids = new Set(selectedAuthors.map((a) => a.qid));

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="author-name-search">Поиск автора по имени</Label>
        <div className="flex gap-2">
          <Input
            id="author-name-search"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Толстой"
          />
          <Button type="button" onClick={handleSearch} disabled={searchMutation.isPending}>
            {searchMutation.isPending ? 'Поиск...' : 'Найти'}
          </Button>
        </div>
      </div>

      {searchError && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {searchError}
        </div>
      )}

      {!searchError && hasSearched && candidates.length === 0 && (
        <p className="text-sm text-muted-foreground">
          Совпадений не найдено. Проверьте написание или используйте ручной ввод QID.
        </p>
      )}

      {candidates.length > 0 && (
        <div className="space-y-2">
          {candidates.map((candidate) => {
            const alreadyAdded = selectedQids.has(candidate.qid);
            return (
              <Card key={candidate.qid}>
                <CardContent className="flex items-center justify-between gap-4 p-4">
                  <div>
                    <div className="font-medium">{candidate.label}</div>
                    <div className="text-sm text-muted-foreground">
                      {candidate.description || 'без описания'}
                    </div>
                  </div>
                  <Button
                    type="button"
                    variant={alreadyAdded ? 'outline' : 'default'}
                    size="sm"
                    disabled={alreadyAdded}
                    onClick={() => onAdd({ qid: candidate.qid, label: candidate.label })}
                  >
                    {alreadyAdded ? 'Добавлено' : 'Добавить'}
                  </Button>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}

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
