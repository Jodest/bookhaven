import { BookDetailView, generateBookMetadata } from '@/views/book-detail';

interface PageProps {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ tab?: string | string[] }>;
}

export async function generateMetadata({ params }: PageProps) {
  const { slug } = await params;
  return generateBookMetadata(slug);
}

export default async function Page({ params, searchParams }: PageProps) {
  const { slug } = await params;
  const { tab } = await searchParams;
  return <BookDetailView slug={slug} tab={Array.isArray(tab) ? tab[0] : tab} />;
}
