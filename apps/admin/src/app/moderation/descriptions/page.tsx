import { Suspense } from 'react';
import { AiDescriptionModerationView } from '@/views/ai-description-moderation';

// The view reads ?status= and ?page= (useSearchParams), which needs a
// Suspense boundary.
export default function Page() {
  return (
    <Suspense fallback={<div className="text-sm text-muted-foreground">Загрузка...</div>}>
      <AiDescriptionModerationView />
    </Suspense>
  );
}
