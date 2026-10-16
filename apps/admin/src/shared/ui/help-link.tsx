import { CircleHelp } from 'lucide-react';
import Link from 'next/link';

/** «?» link to the help chapter of the current section. */
export function HelpLink({ href }: { href: string }) {
  return (
    <Link
      href={href as any}
      aria-label="Справка по разделу"
      title="Справка по разделу"
      className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
    >
      <CircleHelp className="h-5 w-5" />
    </Link>
  );
}
