import { CatalogView } from '@/views/catalog';
import { Suspense } from 'react';

export default function Page() {
  return (
    <Suspense>
      <CatalogView />
    </Suspense>
  );
}
