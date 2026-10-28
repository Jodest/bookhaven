'use client';

import type { MouseEvent, ReactNode } from 'react';
import { BOOK_TABS_ANCHOR_ID, BOOK_TAB_EVENT, replaceTabInUrl, type BookTabId } from '../lib/book-tabs';

interface BookTabLinkProps {
  tab: BookTabId;
  className?: string;
  children: ReactNode;
}

/**
 * Link that selects a book page tab and scrolls the tabs into view. Without
 * JavaScript the href still works: the server renders that tab and the
 * browser jumps to the anchor.
 */
export function BookTabLink({ tab, className, children }: BookTabLinkProps) {
  const onClick = (e: MouseEvent<HTMLAnchorElement>) => {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    replaceTabInUrl(tab);
    window.dispatchEvent(new CustomEvent(BOOK_TAB_EVENT, { detail: tab }));
    const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
    document
      .getElementById(BOOK_TABS_ANCHOR_ID)
      ?.scrollIntoView({ behavior: reduceMotion ? 'auto' : 'smooth', block: 'start' });
  };

  return (
    <a href={`?tab=${tab}#${BOOK_TABS_ANCHOR_ID}`} onClick={onClick} className={className}>
      {children}
    </a>
  );
}
