'use client';

import { Button, Input, Label, Select, Textarea } from '@bookhaven/ui';
import { KIND_LABELS, METRIC_LABELS } from '@/entities/curated-collection';
import { metricPatch, validateRule } from '../lib/collection-form';
import type { CollectionFormValues } from '../lib/collection-form';

const METRIC_OPTIONS = (Object.keys(METRIC_LABELS) as CollectionFormValues['metric'][]).map((value) => ({
  value,
  label: METRIC_LABELS[value],
}));

interface CollectionFormProps {
  values: CollectionFormValues;
  onChange: (patch: Partial<CollectionFormValues>) => void;
  /** Published genres to link; '' = none. */
  genres: { id: string; name: string }[];
  editing: boolean;
  onSubmit: (e: React.FormEvent) => void;
  pending: boolean;
  /** Field error under «Адрес (slug)». */
  slugError?: string;
}

export function CollectionForm({
  values,
  onChange,
  genres,
  editing,
  onSubmit,
  pending,
  slugError,
}: CollectionFormProps) {
  const genreOptions = [{ value: '', label: 'Без жанра' }, ...genres.map((g) => ({ value: g.id, label: g.name }))];

  const isAuto = values.kind === 'auto';
  const ruleError = validateRule(values);

  return (
    <form onSubmit={onSubmit} className="space-y-4 rounded-lg border p-4">
      {editing ? (
        <p className="text-sm">
          <span className="text-muted-foreground">Тип: </span>
          <span className="font-medium">{KIND_LABELS[values.kind]}</span>
        </p>
      ) : (
        <fieldset className="space-y-2">
          <legend className="text-sm font-medium">Тип</legend>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="radio"
              name="kind"
              checked={values.kind === 'manual'}
              onChange={() => onChange({ kind: 'manual' })}
            />
            Ручная — книги и порядок задаёт администратор
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="radio"
              name="kind"
              checked={values.kind === 'auto'}
              onChange={() => onChange({ kind: 'auto' })}
            />
            Автоматическая — по правилу из данных сайта
          </label>
        </fieldset>
      )}
      <div className="space-y-2">
        <Label htmlFor="title">Название</Label>
        <Input id="title" value={values.title} onChange={(e) => onChange({ title: e.target.value })} required />
      </div>
      <div className="space-y-2">
        <Label htmlFor="slug">Адрес (slug)</Label>
        <Input
          id="slug"
          value={values.slug}
          onChange={(e) => onChange({ slug: e.target.value })}
          placeholder={editing ? undefined : 'Автоматически из названия'}
          aria-invalid={Boolean(slugError)}
        />
        {slugError ? (
          <p className="text-xs text-destructive">{slugError}</p>
        ) : (
          <p className="text-xs text-muted-foreground">
            Страница откроется по адресу /top/{values.slug || '…'}. Латиница, цифры и дефисы.
          </p>
        )}
      </div>
      <div className="space-y-2">
        <Label htmlFor="description">Описание</Label>
        <Textarea
          id="description"
          rows={4}
          value={values.description}
          onChange={(e) => onChange({ description: e.target.value })}
        />
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="size">Количество мест</Label>
          <Input
            id="size"
            type="number"
            min={1}
            max={1000}
            value={Number.isNaN(values.size) ? '' : values.size}
            onChange={(e) => onChange({ size: e.target.value === '' ? NaN : Number(e.target.value) })}
            required
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="genre">Жанр</Label>
          <Select
            id="genre"
            options={genreOptions}
            value={values.genre_id}
            onChange={(e) => onChange({ genre_id: e.target.value })}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="source">Источник</Label>
          <Input
            id="source"
            value={values.source}
            onChange={(e) => onChange({ source: e.target.value })}
            placeholder="по версии BBC Big Read, 2003"
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="source_url">Ссылка на источник</Label>
          <Input
            id="source_url"
            type="url"
            value={values.source_url}
            onChange={(e) => onChange({ source_url: e.target.value })}
            placeholder="https://"
          />
        </div>
      </div>
      {isAuto && (
        <fieldset className="space-y-4 rounded-md border p-3">
          <legend className="px-1 text-sm font-medium">Правило</legend>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="metric">Показатель</Label>
              <Select
                id="metric"
                options={METRIC_OPTIONS}
                value={values.metric}
                onChange={(e) => onChange(metricPatch(values, e.target.value as CollectionFormValues['metric']))}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="period_days">Период, дней</Label>
              <Input
                id="period_days"
                type="number"
                min={1}
                max={3650}
                value={values.period_days}
                onChange={(e) => onChange({ period_days: e.target.value })}
                placeholder="за всё время"
              />
            </div>
            {values.metric === 'weighted_rating' && (
              <>
                <div className="space-y-2">
                  <Label htmlFor="min_ratings">Минимум оценок</Label>
                  <Input
                    id="min_ratings"
                    type="number"
                    min={1}
                    max={10000}
                    value={values.min_ratings}
                    onChange={(e) => onChange({ min_ratings: e.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="confidence">Порог доверия (m)</Label>
                  <Input
                    id="confidence"
                    type="number"
                    min={1}
                    max={10000}
                    value={values.confidence}
                    onChange={(e) => onChange({ confidence: e.target.value })}
                  />
                  <p className="text-xs text-muted-foreground">
                    Чем больше m, тем сильнее рейтинг книг с малым числом оценок тянется к среднему по сайту
                  </p>
                </div>
              </>
            )}
          </div>
          {ruleError && (
            <p role="alert" className="text-xs text-destructive">
              {ruleError}
            </p>
          )}
          <p className="text-xs text-muted-foreground">«Жанр» ограничивает подборку книгами этого жанра.</p>
        </fieldset>
      )}
      <div className="flex items-center gap-2">
        <input
          id="is_published"
          type="checkbox"
          checked={values.is_published}
          onChange={(e) => onChange({ is_published: e.target.checked })}
        />
        <Label htmlFor="is_published">Опубликована</Label>
      </div>
      <Button type="submit" disabled={pending || Boolean(ruleError)}>
        {editing ? 'Сохранить' : 'Создать'}
      </Button>
    </form>
  );
}
