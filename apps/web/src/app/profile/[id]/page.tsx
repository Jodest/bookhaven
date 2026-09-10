import { PublicProfileView } from '@/views/public-profile';

interface PageProps {
  params: { id: string };
}

export default function Page({ params }: PageProps) {
  return <PublicProfileView id={params.id} />;
}
