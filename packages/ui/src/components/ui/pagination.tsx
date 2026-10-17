import type { MouseEventHandler, ReactNode } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { getPaginationRange } from '../../lib/pagination-range';
import { cn } from '../../lib/utils';
import { buttonVariants } from './button';

export interface PaginationLinkProps {
  href: string;
  className: string;
  'aria-label': string;
  children: ReactNode;
  onClick?: MouseEventHandler<HTMLAnchorElement>;
}

export interface PaginationProps {
  page: number;
  totalPages: number;
  /** Target of the "previous" control. Defaults to `page - 1`. */
  prevPage?: number;
  /** Target of the "next" control. Defaults to `page + 1`. */
  nextPage?: number;
  /** Link mode: items are links built from this function. */
  getHref?: (page: number) => string;
  /** Custom link renderer (e.g. a Next `Link`). Defaults to a plain `<a>`. */
  renderLink?: (props: PaginationLinkProps) => ReactNode;
  /** Button mode handler; in link mode it is called from the link `onClick`. */
  onPageChange?: (page: number) => void;
  siblings?: number;
  className?: string;
}

const itemClass = 'min-w-9 px-3';

export function Pagination({
  page,
  totalPages,
  prevPage,
  nextPage,
  getHref,
  renderLink,
  onPageChange,
  siblings = 1,
  className,
}: PaginationProps) {
  if (totalPages <= 1) return null;

  const prevTarget = prevPage ?? page - 1;
  const nextTarget = nextPage ?? page + 1;
  const inRange = (p: number) => p >= 1 && p <= totalPages;

  const control = (
    target: number,
    label: string,
    children: ReactNode,
    variant: 'outline' | 'ghost',
    extraClass?: string
  ): ReactNode => {
    const cls = cn(buttonVariants({ variant, size: 'sm' }), extraClass);
    if (!inRange(target)) {
      if (getHref) {
        return (
          <span
            aria-disabled="true"
            aria-label={label}
            role="link"
            className={cn(cls, 'pointer-events-none opacity-50')}
          >
            {children}
          </span>
        );
      }
      return (
        <button type="button" disabled aria-label={label} className={cls}>
          {children}
        </button>
      );
    }
    if (getHref) {
      const onClick: MouseEventHandler<HTMLAnchorElement> | undefined = onPageChange
        ? (e) => {
            // Opening in a new tab/window must not act on the current page.
            if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
            onPageChange(target);
          }
        : undefined;
      const linkProps: PaginationLinkProps = {
        href: getHref(target),
        className: cls,
        'aria-label': label,
        children,
        onClick,
      };
      return renderLink ? (
        renderLink(linkProps)
      ) : (
        <a {...linkProps} />
      );
    }
    return (
      <button type="button" aria-label={label} className={cls} onClick={() => onPageChange?.(target)}>
        {children}
      </button>
    );
  };

  const items = getPaginationRange(page, totalPages, siblings);

  return (
    <nav
      aria-label="Пагинация"
      className={cn('flex items-center justify-center gap-1', className)}
    >
      {control(prevTarget, 'Предыдущая страница', <ChevronLeft className="h-4 w-4" aria-hidden="true" />, 'outline')}
      <ul className="hidden items-center gap-1 sm:flex">
        {items.map((item) =>
          item.type === 'ellipsis' ? (
            <li key={`ellipsis-${item.key}`} aria-hidden="true" className="px-2 text-sm text-muted-foreground">
              …
            </li>
          ) : (
            <li key={item.page}>
              {item.page === page ? (
                <span
                  aria-current="page"
                  aria-label={`Страница ${item.page}`}
                  className={cn(buttonVariants({ variant: 'default', size: 'sm' }), itemClass)}
                >
                  {item.page}
                </span>
              ) : (
                control(item.page, `Страница ${item.page}`, item.page, 'ghost', itemClass)
              )}
            </li>
          )
        )}
      </ul>
      <span
        aria-label={`Страница ${page} из ${totalPages}`}
        className="flex items-center px-3 text-sm sm:hidden"
      >
        {page} / {totalPages}
      </span>
      {control(nextTarget, 'Следующая страница', <ChevronRight className="h-4 w-4" aria-hidden="true" />, 'outline')}
    </nav>
  );
}
