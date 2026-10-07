'use client';

import { useAuthStore } from '@/entities/session';
import { getSectionHref, navSections, type NavSectionId } from '@/shared/config';
import { useRouter } from 'next/navigation';
import { useEffect } from 'react';

/**
 * Bare section path (e.g. `/moderation`) -> first tab visible to the user.
 * Renders nothing; when no tab is visible the Shell guard shows AccessDenied.
 */
export function SectionIndexRedirect({ sectionId }: { sectionId: NavSectionId }) {
  const router = useRouter();
  const { user } = useAuthStore();
  const role = user?.role;
  const section = navSections.find((s) => s.id === sectionId);
  const href = section ? getSectionHref(section, role) : null;

  useEffect(() => {
    if (href) {
      router.replace(href as any);
    }
  }, [href, router]);

  return null;
}
