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

export function generateMetadata({ params }: { params: { chapter: string } }): Metadata {
  const chapter = getHelpChapter(params.chapter);
  return { title: chapter ? `${chapter.title} — Справка` : 'Справка' };
}

export default function Page({ params }: { params: { chapter: string } }) {
  if (!getHelpChapter(params.chapter)) notFound();
  return <HelpChapter markdown={readGuideChapter(params.chapter)} />;
}
