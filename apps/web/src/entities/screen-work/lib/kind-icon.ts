import type { ScreenWorkKind } from '@bookhaven/api-client';
import { Drama, Film, Sparkles, Tv, type LucideIcon } from 'lucide-react';

/** Icon of the poster placeholder, by work kind. */
export const SCREEN_WORK_KIND_ICONS: Record<ScreenWorkKind, LucideIcon> = {
  film: Film,
  tv_film: Film,
  series: Tv,
  animated_film: Sparkles,
  animated_series: Sparkles,
  teleplay: Drama,
};

export function screenWorkKindIcon(kind: ScreenWorkKind): LucideIcon {
  return SCREEN_WORK_KIND_ICONS[kind] ?? Film;
}
