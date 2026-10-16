'use client';

import { useAuthStore } from '@/entities/session';
import { getHelpHref, getVisibleHelpChapters } from '@/shared/config';
import Link from 'next/link';

export function HelpIndex() {
  const { user } = useAuthStore();
  const chapters = getVisibleHelpChapters(user?.role);

  return (
    <div className="max-w-3xl">
      <h1 className="mb-2 text-2xl font-bold">Справка</h1>
      <p className="mb-4 text-muted-foreground">
        Руководство по работе с админкой. Выберите главу: каждая описывает одну задачу от начала до конца.
      </p>
      <ul className="list-disc space-y-1 pl-6">
        {chapters.map((chapter) => (
          <li key={chapter.slug}>
            <Link href={getHelpHref(chapter.slug) as any} className="text-primary underline">
              {chapter.title}
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
