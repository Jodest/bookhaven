import { ShelfDetailView } from '@/views/shelf-detail';

interface PageProps {
  params: { id: string };
}

export default function Page({ params }: PageProps) {
  return <ShelfDetailView id={params.id} />;
}
