import { Suspense } from 'react';
import { GenreMergeHistoryView } from '@/views/genre-merge-history';

// The view reads ?page= and ?undo= (useSearchParams), which needs a Suspense
// boundary.
export default function Page() {
  return (
    <Suspense fallback={<div className="text-sm text-muted-foreground">Загрузка...</div>}>
      <GenreMergeHistoryView />
    </Suspense>
  );
}
