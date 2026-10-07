import type { NavSectionId } from '@/shared/config';
import { SectionIndexRedirect } from '@/widgets/section-tabs';

/** Bare section path page: redirects to the first tab visible to the user. */
export function SectionIndexView({ sectionId }: { sectionId: NavSectionId }) {
  return <SectionIndexRedirect sectionId={sectionId} />;
}
