import { AuthorDetailView } from '@/views/author-detail';

interface PageProps {
  params: Promise<{ slug: string }>;
}

export default async function Page({ params }: PageProps) {
  const { slug } = await params;
  return <AuthorDetailView slug={slug} />;
}
