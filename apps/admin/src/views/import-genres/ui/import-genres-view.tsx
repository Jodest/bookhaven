'use client';

import { WikidataGenreImportPanel } from '@/widgets/wikidata-genre-import';
import { GenreCreateSection } from './genre-create-section';

export type ImportGenresMethod = 'manual' | 'wikidata';

const METHODS: Record<
  ImportGenresMethod,
  { title: string; description: string; Panel: () => React.ReactNode }
> = {
  manual: {
    title: 'Импорт жанров: вручную',
    description: 'Создайте жанр через форму. Он сразу опубликован.',
    Panel: GenreCreateSection,
  },
  wikidata: {
    title: 'Импорт жанров: Wikidata',
    description:
      'Найдите литературный жанр в Wikidata и импортируйте его. Импортированные жанры ждут модерации.',
    Panel: WikidataGenreImportPanel,
  },
};

/**
 * Импорт → Жанры, one sub-page per method: /import/genres/{manual,wikidata}
 * (docs/features/import-method-subpages.md). Genres have no file import.
 */
export function ImportGenresView({ method }: { method: ImportGenresMethod }) {
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
