'use client';

import { Button } from '@bookhaven/ui';
import { useEffect, useId, useRef, useState, type ReactNode } from 'react';

// Keep in sync with the `nth-child(n+5)` class below.
const INITIAL_VISIBLE = 4;
// Must stay a literal string so Tailwind's content scan finds it.
// Compiles to `.cls[data-expanded="false"]>li:nth-child(n+5){display:none}`.
export const HIDE_SURPLUS_CLASS = '[&>li:nth-child(n+5)]:data-[expanded=false]:hidden';

interface CollapsibleCardListProps {
  /** Server-rendered `<li>` nodes (each should have `tabIndex={-1}`). */
  children: ReactNode;
  total: number;
  className?: string;
}

/**
 * List that shows the first 4 items and reveals the rest with
 * «Показать все (N)». Every item stays in the server HTML; the surplus is
 * only hidden with CSS.
 */
export function CollapsibleCardList({
  children,
  total,
  className,
}: CollapsibleCardListProps) {
  const initialVisible = INITIAL_VISIBLE;
  const [expanded, setExpanded] = useState(false);
  const listId = useId();
  const listRef = useRef<HTMLUListElement>(null);
  const focusPending = useRef(false);
  const collapsible = total > initialVisible;

  useEffect(() => {
    if (expanded && focusPending.current) {
      focusPending.current = false;
      (listRef.current?.querySelectorAll(':scope > li')[initialVisible] as HTMLElement | undefined)?.focus();
    }
  }, [expanded, initialVisible]);

  return (
    <>
      <ul
        id={listId}
        ref={listRef}
        data-expanded={expanded}
        className={[
          className,
          collapsible ? HIDE_SURPLUS_CLASS : '',
        ]
          .filter(Boolean)
          .join(' ')}
      >
        {children}
      </ul>
      {collapsible && !expanded && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="mt-3"
          aria-expanded={false}
          aria-controls={listId}
          onClick={() => {
            focusPending.current = true;
            setExpanded(true);
          }}
        >
          Показать все ({total})
        </Button>
      )}
    </>
  );
}
