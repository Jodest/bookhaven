'use client';

import { PageSection } from '@/shared/ui';
import { WikidataGenreImportPanel } from '@/widgets/wikidata-genre-import';
import { GenreCreateSection } from './genre-create-section';

/**
 * Импорт → Жанры (/import/genres, docs/features/import-by-entity.md):
 * Вручную → Wikidata. Genres have no file import.
 */
export function ImportGenresView() {
  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Импорт: жанры</h1>

      <PageSection title="Вручную" description="Создайте жанр через форму. Он сразу опубликован.">
        <GenreCreateSection />
      </PageSection>

      <PageSection
        title="Wikidata"
        description="Найдите литературный жанр в Wikidata и импортируйте его. Импортированные жанры ждут модерации."
      >
        <WikidataGenreImportPanel />
      </PageSection>
    </div>
  );
}
