'use client';

import { BookHarvestPanel } from '@/widgets/book-harvest';
import { BookFileImportPanel } from '@/widgets/book-file-import';
import { BookCreateSection } from './book-create-section';

export type ImportBooksMethod = 'manual' | 'wikidata' | 'file';

const METHODS: Record<
  ImportBooksMethod,
  { title: string; description: string; Panel: () => React.ReactNode }
> = {
  manual: {
    title: 'Импорт книг: вручную',
    description: 'Создайте книгу через форму.',
    Panel: BookCreateSection,
  },
  wikidata: {
    title: 'Импорт книг: харвест Wikidata',
    description: 'Запустите фоновый харвест книг выбранных авторов из Wikidata.',
    Panel: BookHarvestPanel,
  },
  file: {
    title: 'Импорт книг: из файла',
    description: 'Загрузите партию книг из файла .csv или .json.',
    Panel: BookFileImportPanel,
  },
};

/**
 * Импорт → Книги, one sub-page per method: /import/books/{manual,wikidata,file}
 * (docs/features/import-method-subpages.md). Sub-tabs come from the section
 * layout.
 */
export function ImportBooksView({ method }: { method: ImportBooksMethod }) {
  const { title, description, Panel } = METHODS[method];
  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="text-3xl font-bold tracking-tight">{title}</h1>
        <p className="text-sm text-muted-foreground">{description}</p>
      </div>
      <Panel />
    </div>
  );
}
