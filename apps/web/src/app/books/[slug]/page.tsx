import { BookDetailView, generateBookMetadata } from '@/views/book-detail';

interface PageProps {
  params: Promise<{ slug: string }>;
}

export async function generateMetadata({ params }: PageProps) {
  const { slug } = await params;
  return generateBookMetadata(slug);
}

export default async function Page({ params }: PageProps) {
  const { slug } = await params;
  return <BookDetailView slug={slug} />;
}
