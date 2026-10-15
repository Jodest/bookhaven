import type { ScreenWorkKind } from '@bookhaven/api-client';

export const SCREEN_WORK_KIND_LABELS: Record<ScreenWorkKind, string> = {
  film: 'Фильм',
  tv_film: 'Телефильм',
  series: 'Сериал',
  animated_film: 'Мультфильм',
  animated_series: 'Мультсериал',
  teleplay: 'Телеспектакль',
};

export function screenWorkKindLabel(kind: ScreenWorkKind): string {
  return SCREEN_WORK_KIND_LABELS[kind] ?? kind;
}
