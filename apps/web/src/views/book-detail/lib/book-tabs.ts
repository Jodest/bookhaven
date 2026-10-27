/** Tabs of the lower part of the book page (openspec screen-works/book-adaptations). */
export const BOOK_TAB_IDS = ['adaptations', 'reviews', 'similar'] as const;
export type BookTabId = (typeof BOOK_TAB_IDS)[number];

/** id of the element the tab links scroll to. */
export const BOOK_TABS_ANCHOR_ID = 'book-tabs';

/** Window event that asks BookTabs to select a tab (detail: BookTabId). */
export const BOOK_TAB_EVENT = 'bookhaven:select-book-tab';

export function isBookTabId(value: unknown): value is BookTabId {
  return typeof value === 'string' && (BOOK_TAB_IDS as readonly string[]).includes(value);
}

/**
 * The tab the page opens on: the requested one when it exists, else
 * «Экранизации» when the book has them, else «Рецензии».
 */
export function resolveInitialTab(requested: string | undefined, hasAdaptations: boolean): BookTabId {
  if (isBookTabId(requested) && (requested !== 'adaptations' || hasAdaptations)) return requested;
  return hasAdaptations ? 'adaptations' : 'reviews';
}

/** Puts `tab` into the address without navigating, scrolling or adding a history entry. */
export function replaceTabInUrl(tab: BookTabId): void {
  const url = new URL(window.location.href);
  url.searchParams.set('tab', tab);
  window.history.replaceState(window.history.state, '', url);
}
