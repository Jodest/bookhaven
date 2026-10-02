import { PublicShelfDetailView } from '@/views/public-shelf-detail';

interface PageProps {
  params: { id: string };
}

export default function Page({ params }: PageProps) {
  return <PublicShelfDetailView id={params.id} />;
}
