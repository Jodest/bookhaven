import { CollectionDetailView } from '@/views/collection-detail';
import { Suspense } from 'react';

interface PageProps {
  params: { slug: string };
}

export default function Page({ params }: PageProps) {
  return (
    <Suspense>
      <CollectionDetailView slug={params.slug} />
    </Suspense>
  );
}
