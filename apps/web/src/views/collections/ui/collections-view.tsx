'use client';

import { CollectionCard, useCuratedCollections } from '@/entities/curated-collection';

export function CollectionsView() {
  const { data, isLoading, error } = useCuratedCollections();

  return (
    <main className="container mx-auto px-4 py-12 sm:px-6 md:py-16 lg:px-8">
      <h1 className="font-display text-2xl font-semibold tracking-tight md:text-3xl">Подборки</h1>
      <p className="mt-2 text-muted-foreground">Рейтинги и списки лучших книг от BookHaven.</p>

      {isLoading && <p className="mt-8 text-muted-foreground">Загрузка...</p>}
      {error && (
        <div className="mt-8 rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          Не удалось загрузить подборки
        </div>
      )}
      {data && data.items.length === 0 && <p className="mt-8 text-muted-foreground">Подборок пока нет</p>}
      {data && data.items.length > 0 && (
        <div className="mt-8 grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {data.items.map((c) => (
            <CollectionCard key={c.id} collection={c} />
          ))}
        </div>
      )}
    </main>
  );
}
