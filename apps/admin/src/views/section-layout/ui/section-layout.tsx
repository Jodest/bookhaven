import { SectionSubtabs, SectionTabs } from '@/widgets/section-tabs';

/**
 * Section segment layout: navigation-config driven tabs (and, for tabs with
 * sub-pages, the secondary sub-tab row) above the page. The wrapper collapses
 * when neither row renders.
 */
export function SectionLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <div className="mb-6 space-y-3 empty:hidden">
        <SectionTabs />
        <SectionSubtabs />
      </div>
      {children}
    </>
  );
}
