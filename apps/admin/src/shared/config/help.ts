import type { StaffRole } from './navigation';

/**
 * Registry of admin-guide chapters. Slug = file name in docs/admin-guide/<slug>.md.
 * Role scoping is a navigation aid only: the guide is public static content.
 */
export type HelpChapterSlug =
  | 'getting-started'
  | 'adding-content'
  | 'import-jobs'
  | 'collections'
  | 'catalog-review'
  | 'moderation'
  | 'duplicates'
  | 'screen-works'
  | 'users-roles';

export interface HelpChapter {
  slug: HelpChapterSlug;
  title: string;
  roles: StaffRole[];
}

export const helpChapters: HelpChapter[] = [
  { slug: 'getting-started', title: 'Первые шаги', roles: ['admin'] },
  { slug: 'adding-content', title: 'Как добавить книгу, автора или жанр', roles: ['admin'] },
  { slug: 'import-jobs', title: 'Задачи импорта', roles: ['admin'] },
  { slug: 'collections', title: 'Подборки и ИИ-кураторы', roles: ['admin'] },
  { slug: 'catalog-review', title: 'Проверка каталога', roles: ['admin'] },
  { slug: 'moderation', title: 'Модерация отзывов', roles: ['admin', 'moderator'] },
  { slug: 'duplicates', title: 'Слияние дублей', roles: ['admin'] },
  { slug: 'screen-works', title: 'Экранизации', roles: ['admin'] },
  { slug: 'users-roles', title: 'Пользователи и роли', roles: ['admin'] },
];

export function getHelpChapter(slug: string): HelpChapter | null {
  return helpChapters.find((c) => c.slug === slug) ?? null;
}

export function getVisibleHelpChapters(role: string | undefined): HelpChapter[] {
  return helpChapters.filter((c) => !!role && (c.roles as string[]).includes(role));
}

export function getHelpHref(slug: HelpChapterSlug): string {
  return `/help/${slug}`;
}
