import { CollectionDetailView } from '@/views/collection-detail';
import { Suspense } from 'react';

interface PageProps {
  params: Promise<{ slug: string }>;
}

export default async function Page({ params }: PageProps) {
  const { slug } = await params;
  return (
    <Suspense>
      <CollectionDetailView slug={slug} />
    </Suspense>
  );
}
