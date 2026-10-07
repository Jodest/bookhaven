import { SectionTabs } from '@/widgets/section-tabs';

/** Section segment layout: navigation-config driven tabs above the page. */
export function SectionLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <SectionTabs />
      {children}
    </>
  );
}
