'use client';

import { useAuthStore } from '@/entities/session';
import { getHelpHref, getVisibleHelpChapters } from '@/shared/config';
import Link from 'next/link';
import { usePathname } from 'next/navigation';

export function HelpLayout({ children }: { children: React.ReactNode }) {
  const { user } = useAuthStore();
  const pathname = usePathname();
  const chapters = getVisibleHelpChapters(user?.role);

  return (
    <div className="flex flex-col gap-6 md:flex-row">
      <nav aria-label="Главы справки" className="md:w-64 md:shrink-0">
        <ul className="space-y-1">
          {chapters.map((chapter) => {
            const href = getHelpHref(chapter.slug);
            const active = pathname === href;
            return (
              <li key={chapter.slug}>
                <Link
                  href={href as any}
                  aria-current={active ? 'page' : undefined}
                  className={`block rounded-md px-3 py-2 text-sm transition-colors ${
                    active
                      ? 'bg-primary font-medium text-primary-foreground'
                      : 'text-foreground hover:bg-muted'
                  }`}
                >
                  {chapter.title}
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}
