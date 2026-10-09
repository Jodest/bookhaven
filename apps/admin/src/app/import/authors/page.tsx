import { redirect } from 'next/navigation';
import { getTabHref } from '@/shared/config';

// Bare entity path → the tab's default sub-page (the tab href). The HTTP 307
// comes from next.config.js redirects(); this server redirect() is only a
// fallback (it streams a 200 under the client root layout).
// docs/features/import-method-subpages.md
export default function Page() {
  redirect(getTabHref('import.authors') as any);
}
