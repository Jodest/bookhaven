import { CollectionCard } from '@/entities/curated-collection';
import type { CuratedCollection } from '@bookhaven/api-client';
import Link from 'next/link';

interface CuratedCollectionsRowProps {
  collections: CuratedCollection[];
}

/** Home block «Подборки» (openspec home/home-page); renders nothing when empty. */
export function CuratedCollectionsRow({ collections }: CuratedCollectionsRowProps) {
  if (collections.length === 0) return null;

  return (
    <section aria-label="Подборки">
      <div className="flex items-baseline justify-between gap-4">
        <h2 className="font-display text-xl font-semibold md:text-2xl">Подборки</h2>
        <Link href="/top" className="shrink-0 text-sm font-medium text-primary hover:underline">
          Все подборки
        </Link>
      </div>
      <div className="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {collections.map((c) => (
          <CollectionCard key={c.id} collection={c} headingLevel="h3" />
        ))}
      </div>
    </section>
  );
}
