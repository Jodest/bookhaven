'use client';

import { useSearchParams } from 'next/navigation';
import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import {
  BOOK_TABS_ANCHOR_ID,
  BOOK_TAB_EVENT,
  isBookTabId,
  replaceTabInUrl,
  type BookTabId,
} from '../lib/book-tabs';

export interface BookTabDef {
  id: BookTabId;
  label: string;
  /** Rendered by the server view, so the adaptations stay server-rendered HTML. */
  panel: ReactNode;
}

interface BookTabsProps {
  tabs: BookTabDef[];
  initialTab: BookTabId;
}

/**
 * Accessible tabs of the book page. Every panel is always rendered; the
 * inactive ones get the `hidden` attribute, so their content stays in the
 * server-rendered HTML. The selected tab lives in `?tab=` (changed with
 * history.replaceState: no navigation, scroll or history entry).
 */
export function BookTabs({ tabs, initialTab }: BookTabsProps) {
  const searchParams = useSearchParams();
  const ids = tabs.map((t) => t.id);
  const requested = searchParams?.get('tab') ?? null;
  const fromUrl = isBookTabId(requested) && ids.includes(requested) ? requested : null;
  const [selected, setSelected] = useState<BookTabId>(fromUrl ?? initialTab);
  const tabRefs = useRef<Record<string, HTMLButtonElement | null>>({});

  // `?tab=` changed from outside (a tab link, browser navigation).
  useEffect(() => {
    if (fromUrl) setSelected(fromUrl);
  }, [fromUrl]);

  const select = useCallback(
    (id: BookTabId, focus = false) => {
      if (!ids.includes(id)) return;
      setSelected(id);
      replaceTabInUrl(id);
      if (focus) tabRefs.current[id]?.focus();
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [ids.join('|')],
  );

  useEffect(() => {
    // Old #reviews links.
    if (window.location.hash === '#reviews' && ids.includes('reviews')) {
      setSelected('reviews');
      document.getElementById('reviews')?.scrollIntoView();
    }
    const onSelect = (e: Event) => {
      const id = (e as CustomEvent<unknown>).detail;
      if (isBookTabId(id) && ids.includes(id)) setSelected(id);
    };
    window.addEventListener(BOOK_TAB_EVENT, onSelect);
    return () => window.removeEventListener(BOOK_TAB_EVENT, onSelect);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ids.join('|')]);

  const onKeyDown = (e: KeyboardEvent<HTMLButtonElement>, index: number) => {
    let next: number;
    if (e.key === 'ArrowRight') next = (index + 1) % tabs.length;
    else if (e.key === 'ArrowLeft') next = (index - 1 + tabs.length) % tabs.length;
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = tabs.length - 1;
    else return;
    e.preventDefault();
    select(tabs[next].id, true);
  };

  return (
    <div>
      <div
        id={BOOK_TABS_ANCHOR_ID}
        role="tablist"
        aria-label="Разделы книги"
        className="scroll-mt-24 flex flex-wrap gap-x-6 border-b"
      >
        {tabs.map((tab, index) => {
          const active = tab.id === selected;
          return (
            <button
              key={tab.id}
              ref={(el) => {
                tabRefs.current[tab.id] = el;
              }}
              type="button"
              role="tab"
              id={`book-tab-${tab.id}`}
              aria-selected={active}
              aria-controls={`book-tabpanel-${tab.id}`}
              tabIndex={active ? 0 : -1}
              onClick={() => select(tab.id)}
              onKeyDown={(e) => onKeyDown(e, index)}
              className={[
                '-mb-px border-b-2 py-2 text-sm font-medium transition-colors sm:text-base',
                active
                  ? 'border-primary text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground',
              ].join(' ')}
            >
              {tab.label}
            </button>
          );
        })}
      </div>
      {tabs.map((tab) => (
        <div
          key={tab.id}
          role="tabpanel"
          id={`book-tabpanel-${tab.id}`}
          aria-labelledby={`book-tab-${tab.id}`}
          tabIndex={0}
          hidden={tab.id !== selected}
          className="pt-6"
        >
          {tab.panel}
        </div>
      ))}
    </div>
  );
}
