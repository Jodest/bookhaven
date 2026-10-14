import { Suspense } from 'react';
import { ImportAuthorsView } from '@/views/import-authors';

// The curation widget reads ?request= (useSearchParams), which needs a
// Suspense boundary.
export default function Page() {
  return (
    <Suspense fallback={<div className="text-sm text-muted-foreground">Загрузка...</div>}>
      <ImportAuthorsView method="ai" />
    </Suspense>
  );
}
