'use client';

import { useAuthStore } from '@/entities/session';
import { NavBadge, useNavCounters } from '@/features/nav-counters';
import { getVisibleTabs, resolveNavLocation } from '@/shared/config';
import Link from 'next/link';
import { usePathname } from 'next/navigation';

// Screen-reader label of the tab badge per tab id (tabs without a counter
// have no entry).
const TAB_BADGE_SR_LABELS: Partial<Record<string, string>> = {
  'moderation.books': 'ожидают проверки',
  'moderation.authors': 'ожидают проверки',
  'import.jobs': 'задач в работе',
};

/**
 * Tabs of the section owning the current pathname (navigation config driven).
 * Renders nothing when the section has fewer than two tabs visible to the user.
 */
export function SectionTabs() {
  const pathname = usePathname();
  const { user } = useAuthStore();
  const role = user?.role;
  // Admin-only badge counts; called before the early returns (hooks order).
  const counters = useNavCounters();

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
            <NavBadge
              count={counters.tabs[tab.id]}
              srLabel={TAB_BADGE_SR_LABELS[tab.id] ?? ''}
              className="ml-2"
            />
          </Link>
        );
      })}
    </nav>
  );
}
