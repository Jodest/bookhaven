// Transitional legacy layout (admin navigation restructure, #284): delete
// together with this route when its pages move (#285-#287).
import { SectionLayout } from '@/views/section-layout';

export default function Layout({ children }: { children: React.ReactNode }) {
  return <SectionLayout>{children}</SectionLayout>;
}
