'use client';

import { PageSection } from '@/shared/ui';
import { WikidataAuthorImportPanel } from '@/widgets/wikidata-author-import';
import { AuthorCreateSection } from './author-create-section';

/**
 * Импорт → Авторы (/import/authors, docs/features/import-by-entity.md):
 * Вручную → Wikidata. Authors have no file import.
 */
export function ImportAuthorsView() {
  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Импорт: авторы</h1>

      <PageSection title="Вручную" description="Создайте автора через форму. Он сразу опубликован.">
        <AuthorCreateSection />
      </PageSection>

      <PageSection
        title="Wikidata"
        description="Найдите автора в Wikidata и импортируйте его (с книгами или без). Импортированные авторы ждут модерации."
      >
        <WikidataAuthorImportPanel />
      </PageSection>
    </div>
  );
}
