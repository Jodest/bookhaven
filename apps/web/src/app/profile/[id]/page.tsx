import { PublicProfileView } from '@/views/public-profile';

interface PageProps {
  params: Promise<{ id: string }>;
}

export default async function Page({ params }: PageProps) {
  const { id } = await params;
  return <PublicProfileView id={id} />;
}
