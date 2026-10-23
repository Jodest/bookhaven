import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { getHelpChapter, helpChapters } from '@/shared/config';
import { readGuideChapter } from '@/shared/lib/admin-guide';
import { HelpChapter } from '@/views/help';

export const dynamicParams = false;
export const dynamic = 'force-static';

export function generateStaticParams() {
  return helpChapters.map((c) => ({ chapter: c.slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ chapter: string }>;
}): Promise<Metadata> {
  const { chapter: slug } = await params;
  const chapter = getHelpChapter(slug);
  return { title: chapter ? `${chapter.title} — Справка` : 'Справка' };
}

export default async function Page({ params }: { params: Promise<{ chapter: string }> }) {
  const { chapter } = await params;
  if (!getHelpChapter(chapter)) notFound();
  return <HelpChapter markdown={readGuideChapter(chapter)} />;
}
