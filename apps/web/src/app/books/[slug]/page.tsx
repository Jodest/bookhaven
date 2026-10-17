import { BookDetailView, generateBookMetadata } from '@/views/book-detail';

interface PageProps {
  params: { slug: string };
}

export async function generateMetadata({ params }: PageProps) {
  return generateBookMetadata(params.slug);
}

export default function Page({ params }: PageProps) {
  return <BookDetailView slug={params.slug} />;
}
