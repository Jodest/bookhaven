import { ShelfDetailView } from '@/views/shelf-detail';

interface PageProps {
  params: Promise<{ id: string }>;
}

export default async function Page({ params }: PageProps) {
  const { id } = await params;
  return <ShelfDetailView id={id} />;
}
