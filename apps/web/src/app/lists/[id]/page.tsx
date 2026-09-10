import { ListDetailView } from '@/views/list-detail';

interface PageProps {
  params: { id: string };
}

export default function Page({ params }: PageProps) {
  return <ListDetailView id={params.id} />;
}
