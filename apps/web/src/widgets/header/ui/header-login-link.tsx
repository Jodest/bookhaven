'use client';

import { Button } from '@bookhaven/ui';
import Link from 'next/link';
import { usePathname, useSearchParams } from 'next/navigation';

/**
 * Renders the generic "Войти" (Log in) link, passing the current
 * path + query string as `?redirect=` so a successful login returns
 * the user to the page they were on.
 *
 * Isolated into its own component (rather than inlined in `Header`)
 * because it calls `useSearchParams()`, which requires a `<Suspense>`
 * boundary to avoid opting the whole page into dynamic rendering.
 */
export function HeaderLoginLink() {
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const search = searchParams.toString();
  const currentPath = `${pathname}${search ? `?${search}` : ''}`;

  return (
    <Button variant="ghost" asChild>
      <Link href={`/login?redirect=${encodeURIComponent(currentPath)}`}>Войти</Link>
    </Button>
  );
}
