'use client';

import type { AdminCuratedCollectionDetail } from '@bookhaven/api-client';
import { Button } from '@bookhaven/ui';
import { BookStatusBadge } from '@/entities/book';
import {
  ITEM_STATE_LABELS,
  adaptationSourceLabel,
  formatAdminMovement,
  formatScore,
} from '@/entities/curated-collection';
import { recomputeErrorText, useRecomputeCollection } from '@/features/recompute-collection';

interface CollectionSnapshotProps {
  collection: AdminCuratedCollectionDetail;
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString('ru-RU');
}

/**
 * Read-only view of an automatic collection: when it was computed, the
 * «Пересчитать сейчас» action and the snapshot in order. Books come from the
 * rule, so there are no add, remove or reorder controls.
 */
export function CollectionSnapshot({ collection }: CollectionSnapshotProps) {
  const recompute = useRecomputeCollection(collection.id);
  const metric = collection.rule?.metric ?? 'weighted_rating';
  const error = recomputeErrorText(recompute.error);

  return (
    <section className="space-y-4 rounded-lg border p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1 text-sm">
          <p className="font-medium">
            {collection.computed_at ? `Пересчитано: ${formatDateTime(collection.computed_at)}` : 'Ещё не пересчитана'}
          </p>
          {collection.baseline_at && (
            <p className="text-muted-foreground">
              Движение — относительно пересчёта {formatDateTime(collection.baseline_at)}
            </p>
          )}
        </div>
        <Button type="button" variant="outline" disabled={recompute.isPending} onClick={() => recompute.mutate()}>
          {recompute.isPending ? 'Пересчёт…' : 'Пересчитать сейчас'}
        </Button>
      </div>

      {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}

      {collection.items.length === 0 ? (
        <p className="text-sm text-muted-foreground">В снимке пока нет книг.</p>
      ) : (
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="border-b text-xs uppercase text-muted-foreground">
              <th className="p-2 font-medium">Место</th>
              <th className="p-2 font-medium">Движение</th>
              <th className="p-2 font-medium">Книга</th>
              <th className="p-2 text-right font-medium">Показатель</th>
            </tr>
          </thead>
          <tbody>
            {collection.items.map((item) => (
              <tr key={item.book.id} className="border-b" data-state={item.state}>
                <td className="w-40 p-2 align-top">
                  {item.state === 'ranked' ? (
                    <span className="font-semibold">№ {item.place}</span>
                  ) : (
                    <span className="text-muted-foreground">{ITEM_STATE_LABELS[item.state]}</span>
                  )}
                </td>
                <td className="w-24 p-2 align-top">{formatAdminMovement(item.movement)}</td>
                <td className="p-2 align-top">
                  <div className="font-medium">{item.book.title}</div>
                  <div className="text-muted-foreground">{item.book.authors.map((a) => a.name).join(', ')}</div>
                  {item.adaptation && (
                    <div className="text-muted-foreground">{adaptationSourceLabel(item.adaptation)}</div>
                  )}
                  <div className="mt-1">
                    <BookStatusBadge status={item.book.status} />
                  </div>
                </td>
                <td className="w-28 p-2 text-right align-top tabular-nums">{formatScore(metric, item.score)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}
