'use client';

import { useAuthStore } from '@/entities/session';
import { getVisibleSubtabs, resolveNavLocation } from '@/shared/config';
import Link from 'next/link';
import { usePathname } from 'next/navigation';

/**
 * Secondary pill row with the sub-pages of the active tab (e.g. import
 * methods, docs/features/import-method-subpages.md). Navigation-config driven;
 * renders nothing when the tab has fewer than two sub-tabs visible to the user.
 */
export function SectionSubtabs() {
  const pathname = usePathname();
  const { user } = useAuthStore();

  const loc = resolveNavLocation(pathname);
  if (!loc?.tab) return null;

  const subtabs = getVisibleSubtabs(loc.tab, user?.role);
  if (subtabs.length < 2) return null;

  return (
    <nav aria-label="Способ импорта" className="inline-flex flex-wrap gap-1 rounded-md bg-muted p-1">
      {subtabs.map((subtab) => {
        const active = loc.subtab?.id === subtab.id;
        return (
          <Link
            key={subtab.id}
            href={subtab.href as any}
            aria-current={active ? 'page' : undefined}
            className={`rounded-sm px-3 py-1.5 text-sm font-medium transition-colors ${
              active
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            {subtab.label}
          </Link>
        );
      })}
    </nav>
  );
}
