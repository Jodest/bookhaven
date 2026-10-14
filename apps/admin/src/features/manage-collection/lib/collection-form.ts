import { ApiError } from '@bookhaven/api-client';
import type {
  AdminCuratedCollection,
  AdminCuratedCollectionInput,
  AutoMetric,
  AutoRuleInput,
  CuratedCollectionKind,
} from '@bookhaven/api-client';

/** Form state; every field is controlled, so optional ones are plain strings. */
export interface CollectionFormValues {
  /** Chosen on create, read-only afterwards. */
  kind: CuratedCollectionKind;
  title: string;
  slug: string;
  description: string;
  size: number;
  source: string;
  source_url: string;
  /** '' = no genre. */
  genre_id: string;
  is_published: boolean;
  /** Rule fields, used only for automatic collections. */
  metric: AutoMetric;
  /** '' = all time. */
  period_days: string;
  min_ratings: string;
  confidence: string;
}

export const DEFAULT_MIN_RATINGS = '3';
export const DEFAULT_CONFIDENCE = '10';
export const DEFAULT_TRENDING_DAYS = '30';

export const EMPTY_COLLECTION_FORM: CollectionFormValues = {
  kind: 'manual',
  title: '',
  slug: '',
  description: '',
  size: 100,
  source: '',
  source_url: '',
  genre_id: '',
  is_published: false,
  metric: 'weighted_rating',
  period_days: '',
  min_ratings: DEFAULT_MIN_RATINGS,
  confidence: DEFAULT_CONFIDENCE,
};

export function collectionToForm(c: AdminCuratedCollection): CollectionFormValues {
  return {
    kind: c.kind,
    title: c.title,
    slug: c.slug,
    description: c.description,
    size: c.size,
    source: c.source ?? '',
    source_url: c.source_url ?? '',
    genre_id: c.genre?.id ?? '',
    is_published: c.is_published,
    metric: c.rule?.metric ?? 'weighted_rating',
    period_days: c.rule?.period_days != null ? String(c.rule.period_days) : '',
    min_ratings: c.rule?.min_ratings != null ? String(c.rule.min_ratings) : DEFAULT_MIN_RATINGS,
    confidence: c.rule?.confidence != null ? String(c.rule.confidence) : DEFAULT_CONFIDENCE,
  };
}

/** Patch for a metric change: «В тренде» prefills the 30-day period when none is set. */
export function metricPatch(values: CollectionFormValues, metric: AutoMetric): Partial<CollectionFormValues> {
  const patch: Partial<CollectionFormValues> = { metric };
  if (metric === 'trending' && values.period_days.trim() === '') patch.period_days = DEFAULT_TRENDING_DAYS;
  return patch;
}

function parseInteger(raw: string): number {
  const text = raw.trim();
  return /^\d+$/.test(text) ? Number(text) : NaN;
}

function inRange(n: number, max: number): boolean {
  return Number.isInteger(n) && n >= 1 && n <= max;
}

/** Client-side rule check (UX only; the server validates again). Null when valid or manual. */
export function validateRule(v: CollectionFormValues): string | null {
  if (v.kind !== 'auto') return null;
  if (v.period_days.trim() !== '' && !inRange(parseInteger(v.period_days), 3650)) {
    return 'Период: от 1 до 3650 дней';
  }
  if (v.metric === 'weighted_rating') {
    if (!inRange(parseInteger(v.min_ratings), 10000)) return 'Минимум оценок: от 1 до 10000';
    if (!inRange(parseInteger(v.confidence), 10000)) return 'Порог доверия: от 1 до 10000';
  }
  return null;
}

/** The rule body of an automatic collection; min ratings and confidence only for the weighted rating. */
export function formToRule(v: CollectionFormValues): AutoRuleInput {
  const rule: AutoRuleInput = {
    metric: v.metric,
    period_days: v.period_days.trim() === '' ? null : parseInteger(v.period_days),
  };
  if (v.metric === 'weighted_rating') {
    rule.min_ratings = parseInteger(v.min_ratings);
    rule.confidence = parseInteger(v.confidence);
  }
  return rule;
}

/** Create body: empty optional fields are left out (an empty slug is derived from the title). */
export function formToCreateInput(v: CollectionFormValues): AdminCuratedCollectionInput {
  const input: AdminCuratedCollectionInput = {
    title: v.title.trim(),
    size: v.size,
    is_published: v.is_published,
  };
  if (v.kind === 'auto') {
    input.kind = 'auto';
    input.rule = formToRule(v);
  }
  if (v.slug.trim()) input.slug = v.slug.trim();
  if (v.description.trim()) input.description = v.description;
  if (v.source.trim()) input.source = v.source;
  if (v.source_url.trim()) input.source_url = v.source_url;
  if (v.genre_id) input.genre_id = v.genre_id;
  return input;
}

/**
 * Update body: sends every field so an emptied one clears (genre_id null, '' for texts).
 * The kind is never sent (it cannot change); an automatic collection resends its whole rule.
 */
export function formToUpdateInput(v: CollectionFormValues): AdminCuratedCollectionInput {
  const input: AdminCuratedCollectionInput = {
    title: v.title.trim(),
    slug: v.slug.trim(),
    size: v.size,
    description: v.description,
    source: v.source,
    source_url: v.source_url,
    genre_id: v.genre_id || null,
    is_published: v.is_published,
  };
  if (v.kind === 'auto') input.rule = formToRule(v);
  return input;
}

/** A 409 on a collection create/update is always the slug. */
export function isSlugConflict(err: unknown): boolean {
  return err instanceof ApiError && err.status === 409;
}

/** Text of a create/update/delete error (409 slug becomes «Такой адрес уже занят»). */
export function collectionErrorText(err: unknown): string | undefined {
  if (!err) return undefined;
  if (isSlugConflict(err)) return 'Такой адрес уже занят';
  return err instanceof Error ? err.message : undefined;
}
