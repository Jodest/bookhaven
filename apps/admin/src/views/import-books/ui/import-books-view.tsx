'use client';

import { PageSection } from '@/shared/ui';
import { BookHarvestPanel } from '@/widgets/book-harvest';
import { BookFileImportPanel } from '@/widgets/book-file-import';
import { BookCreateSection } from './book-create-section';

/**
 * Импорт → Книги (/import/books, docs/features/import-by-entity.md): every way
 * to add books, stacked in the fixed order Вручную → Wikidata → Файл.
 */
export function ImportBooksView() {
  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Импорт: книги</h1>

      <PageSection title="Вручную" description="Создайте книгу через форму.">
        <BookCreateSection />
      </PageSection>

      <PageSection
        title="Харвест из Wikidata"
        description="Запустите фоновый харвест книг выбранных авторов из Wikidata."
      >
        <BookHarvestPanel />
      </PageSection>

      <PageSection title="Из файла" description="Загрузите партию книг из файла .csv или .json.">
        <BookFileImportPanel />
      </PageSection>
    </div>
  );
}
