'use client';

import { Input, Label, Select } from '@bookhaven/ui';
import type { ScreenWorkKind } from '@bookhaven/api-client';
import { KIND_OPTIONS } from '../lib/kind-labels';
import type { WorkFormState } from '../lib/work-form';

interface ScreenWorkFieldsProps {
  value: WorkFormState;
  onChange: (patch: Partial<WorkFormState>) => void;
  /** The work already has a QID: it can not be changed. */
  qidReadOnly?: boolean;
  /** Prefix keeping ids unique when several forms are on one page. */
  idPrefix?: string;
}

// Controlled work fields shared by the edit page and the add dialog; no
// mutation of its own.
export function ScreenWorkFields({ value, onChange, qidReadOnly = false, idPrefix = 'work' }: ScreenWorkFieldsProps) {
  const id = (name: string) => `${idPrefix}-${name}`;
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <div className="space-y-2 md:col-span-2">
        <Label htmlFor={id('title')}>Название *</Label>
        <Input id={id('title')} value={value.title} onChange={(e) => onChange({ title: e.target.value })} />
      </div>
      <div className="space-y-2 md:col-span-2">
        <Label htmlFor={id('original-title')}>Оригинальное название</Label>
        <Input
          id={id('original-title')}
          value={value.original_title}
          onChange={(e) => onChange({ original_title: e.target.value })}
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor={id('kind')}>Тип *</Label>
        <Select
          id={id('kind')}
          options={KIND_OPTIONS}
          value={value.kind}
          onChange={(e) => onChange({ kind: e.target.value as ScreenWorkKind })}
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor={id('year')}>Год</Label>
        <Input
          id={id('year')}
          inputMode="numeric"
          value={value.year}
          onChange={(e) => onChange({ year: e.target.value })}
        />
      </div>
      <div className="space-y-2 md:col-span-2">
        <Label htmlFor={id('director')}>Режиссёр</Label>
        <Input id={id('director')} value={value.director} onChange={(e) => onChange({ director: e.target.value })} />
      </div>
      <div className="space-y-2">
        <Label htmlFor={id('imdb')}>IMDb id</Label>
        <Input
          id={id('imdb')}
          value={value.imdb_id}
          onChange={(e) => onChange({ imdb_id: e.target.value })}
          placeholder="tt0069293 или ссылка"
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor={id('kinopoisk')}>Кинопоиск id</Label>
        <Input
          id={id('kinopoisk')}
          value={value.kinopoisk_id}
          onChange={(e) => onChange({ kinopoisk_id: e.target.value })}
          placeholder="43911 или ссылка"
        />
      </div>
      <div className="space-y-2">
        <Label htmlFor={id('qid')}>QID Wikidata</Label>
        <Input
          id={id('qid')}
          value={value.qid}
          readOnly={qidReadOnly}
          onChange={(e) => onChange({ qid: e.target.value })}
          placeholder="Например, Q181069"
        />
      </div>
    </div>
  );
}
