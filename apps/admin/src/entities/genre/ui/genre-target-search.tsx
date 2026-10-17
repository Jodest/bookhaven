'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AdminGenre } from '@bookhaven/api-client';
import { Button, Input } from '@bookhaven/ui';
import { adminGenresApi } from '../api';
import { GenreStatusBadge } from './genre-status-badge';

interface GenreTargetSearchProps {
  // The genre picked so far, or null while the admin is still searching.
  value: AdminGenre | null;
  onChange: (genre: AdminGenre | null) => void;
  // Genres that must not be offered (the ones already in the merge).
  excludeIds: string[];
}

// GenreTargetSearch picks one genre by name (adminGenresApi.list search): a
// text input with a result list, then the picked genre with «Изменить». Used
// by the merge dialogs for «Другой жанр…».
export function GenreTargetSearch({ value, onChange, excludeIds }: GenreTargetSearchProps) {
  const [search, setSearch] = useState('');
  const trimmedSearch = search.trim();
  const searching = value === null;

  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'genres', 'merge-candidates', trimmedSearch],
    queryFn: () => adminGenresApi.list({ search: trimmedSearch, limit: 20 }),
    enabled: searching && trimmedSearch.length > 0,
  });

  const excluded = new Set(excludeIds);
  const candidates = (data?.items ?? []).filter((g) => !excluded.has(g.id));

  if (value) {
    return (
      <div className="flex items-center justify-between rounded-md border p-2">
        <span className="flex items-center gap-2">
          <span className="font-medium">{value.name}</span>
          <GenreStatusBadge status={value.status} />
        </span>
        <Button type="button" variant="ghost" size="sm" onClick={() => onChange(null)}>
          Изменить
        </Button>
      </div>
    );
  }

  return (
    <>
      <Input
        aria-label="Поиск целевого жанра"
        placeholder="Поиск жанра по названию"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      {trimmedSearch.length > 0 && (
        <div className="max-h-48 overflow-y-auto rounded-md border">
          {isLoading ? (
            <div className="p-2 text-sm text-muted-foreground">Поиск...</div>
          ) : candidates.length === 0 ? (
            <div className="p-2 text-sm text-muted-foreground">Жанры не найдены.</div>
          ) : (
            candidates.map((g) => (
              <button
                key={g.id}
                type="button"
                aria-label={g.name}
                className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent"
                onClick={() => onChange(g)}
              >
                <span>{g.name}</span>
                <GenreStatusBadge status={g.status} />
              </button>
            ))
          )}
        </div>
      )}
    </>
  );
}
