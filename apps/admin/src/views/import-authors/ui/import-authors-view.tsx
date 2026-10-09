'use client';

import { WikidataAuthorImportPanel } from '@/widgets/wikidata-author-import';
import { AuthorCreateSection } from './author-create-section';

export type ImportAuthorsMethod = 'manual' | 'wikidata';

const METHODS: Record<
  ImportAuthorsMethod,
  { title: string; description: string; Panel: () => React.ReactNode }
> = {
  manual: {
    title: 'Импорт авторов: вручную',
    description: 'Создайте автора через форму. Он сразу опубликован.',
    Panel: AuthorCreateSection,
  },
  wikidata: {
    title: 'Импорт авторов: Wikidata',
    description:
      'Найдите автора в Wikidata и импортируйте его (с книгами или без). Импортированные авторы ждут модерации.',
    Panel: WikidataAuthorImportPanel,
  },
};

/**
 * Импорт → Авторы, one sub-page per method: /import/authors/{manual,wikidata}
 * (docs/features/import-method-subpages.md). Authors have no file import.
 */
export function ImportAuthorsView({ method }: { method: ImportAuthorsMethod }) {
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
