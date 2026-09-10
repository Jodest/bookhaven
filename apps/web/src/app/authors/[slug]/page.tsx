import { AuthorDetailView } from '@/views/author-detail';

interface PageProps {
  params: { slug: string };
}

export default function Page({ params }: PageProps) {
  return <AuthorDetailView slug={params.slug} />;
}
