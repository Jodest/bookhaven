'use client';

import { useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AdminGenre } from '@bookhaven/api-client';
import { adminGenresApi } from '../api';

interface MergePreviewSummaryProps {
  sourceIds: string[];
  target: AdminGenre;
  // Reports whether the preview has loaded, so the parent can gate confirm.
  onReadyChange: (ready: boolean) => void;
  // Optional: reports whether the preview finished, loaded or failed (the
  // bulk apply dialog waits for every item to settle).
  onSettledChange?: (settled: boolean) => void;
  // Maps a failed preview to the dialog's own error text (same as a merge).
  errorMessage: (err: unknown) => string;
}

// MergePreviewSummary shows what a merge would do before the admin confirms:
// the number of books that move and the curated collections that get
// switched to the target. Read-only (adminGenresApi.mergePreview); always
// fetched fresh (staleTime 0) because counts change between openings.
export function MergePreviewSummary({
  sourceIds,
  target,
  onReadyChange,
  onSettledChange,
  errorMessage,
}: MergePreviewSummaryProps) {
  const { data, isLoading, isSuccess, isError, error } = useQuery({
    queryKey: ['admin', 'genres', 'merge-preview', [...sourceIds].sort(), target.id],
    queryFn: () => adminGenresApi.mergePreview(sourceIds, target.id),
    staleTime: 0,
  });

  useEffect(() => {
    onReadyChange(isSuccess);
    return () => onReadyChange(false);
  }, [isSuccess, onReadyChange]);

  useEffect(() => {
    onSettledChange?.(isSuccess || isError);
    return () => onSettledChange?.(false);
  }, [isSuccess, isError, onSettledChange]);

  if (isLoading) {
    return <div className="text-sm text-muted-foreground">Считаем последствия…</div>;
  }
  if (error) {
    return (
      <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{errorMessage(error)}</div>
    );
  }
  if (!data) return null;

  return (
    <div className="space-y-1 rounded-md border p-3 text-sm">
      <p>Будет перенесено книг: {data.books_reassigned}.</p>
      {data.collections.map((c) => (
        <p key={c.id} className="text-amber-900">
          Подборка «{c.title}» будет переключена на жанр «{target.name}».
        </p>
      ))}
    </div>
  );
}
