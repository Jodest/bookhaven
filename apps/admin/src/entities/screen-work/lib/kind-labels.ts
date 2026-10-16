import type { ScreenWorkKind } from '@bookhaven/api-client';

// Single source of the work kind labels (admin lists, sections, forms).
export const KIND_LABELS: Record<ScreenWorkKind, string> = {
  film: 'Фильм',
  tv_film: 'Телефильм',
  series: 'Сериал',
  animated_film: 'Мультфильм',
  animated_series: 'Мультсериал',
  teleplay: 'Телеспектакль',
};

export const KIND_OPTIONS = (Object.keys(KIND_LABELS) as ScreenWorkKind[]).map((value) => ({
  value,
  label: KIND_LABELS[value],
}));

export function kindLabel(kind: string): string {
  return KIND_LABELS[kind as ScreenWorkKind] ?? kind;
}
