'use client';

import { useAuthStore } from '@/entities/session';
import { getVisibleTabs, resolveNavLocation } from '@/shared/config';
import Link from 'next/link';
import { usePathname } from 'next/navigation';

/**
 * Tabs of the section owning the current pathname (navigation config driven).
 * Renders nothing when the section has fewer than two tabs visible to the user.
 */
export function SectionTabs() {
  const pathname = usePathname();
  const { user } = useAuthStore();
  const role = user?.role;

  const loc = resolveNavLocation(pathname);
  if (!loc) return null;

  const tabs = getVisibleTabs(loc.section, role);
  if (tabs.length < 2) return null;

  return (
    <nav aria-label={loc.section.label} className="mb-6 flex flex-wrap gap-1 border-b">
      {tabs.map((tab) => {
        const active = loc.tab?.id === tab.id;
        return (
          <Link
            key={tab.id}
            href={tab.href as any}
            aria-current={active ? 'page' : undefined}
            className={`-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors ${
              active
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:border-muted hover:text-foreground'
            }`}
          >
            {tab.label}
          </Link>
        );
      })}
    </nav>
  );
}
