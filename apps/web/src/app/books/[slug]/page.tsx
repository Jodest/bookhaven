import { BookDetailView } from '@/views/book-detail';

interface PageProps {
  params: { slug: string };
}

export default function Page({ params }: PageProps) {
  return <BookDetailView slug={params.slug} />;
}
