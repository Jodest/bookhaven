import { PublicListDetailView } from '@/views/public-list-detail';

interface PageProps {
  params: { id: string };
}

export default function Page({ params }: PageProps) {
  return <PublicListDetailView id={params.id} />;
}
