import Link from 'next/link';
import type { ComponentProps } from 'react';
import ReactMarkdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { getHelpChapter } from '@/shared/config';

const CHAPTER_LINK = /^(?:\.\/)?([a-z0-9-]+)\.md(#[\w-]*)?$/;
const LINK_CLASS = 'text-primary underline';

function GuideLink({ href, children }: ComponentProps<'a'>) {
  if (!href) return <span>{children}</span>;
  const chapter = CHAPTER_LINK.exec(href);
  if (chapter && getHelpChapter(chapter[1])) {
    return (
      <Link href={`/help/${chapter[1]}${chapter[2] ?? ''}` as any} className={LINK_CLASS}>
        {children}
      </Link>
    );
  }
  if (/^https?:\/\//i.test(href)) {
    return (
      <a href={href} target="_blank" rel="noopener noreferrer" className={LINK_CLASS}>
        {children}
      </a>
    );
  }
  if (href.startsWith('#')) {
    return (
      <a href={href} className={LINK_CLASS}>
        {children}
      </a>
    );
  }
  // Unknown relative target: keep it as plain text rather than a broken link.
  return <span>{children}</span>;
}

const components: Components = {
  h1: ({ children }) => <h1 className="mb-4 text-2xl font-bold">{children}</h1>,
  h2: ({ children }) => <h2 className="mb-2 mt-8 text-xl font-semibold">{children}</h2>,
  h3: ({ children }) => <h3 className="mb-2 mt-6 text-lg font-semibold">{children}</h3>,
  p: ({ children }) => <p className="my-3 leading-relaxed">{children}</p>,
  ul: ({ children }) => <ul className="my-3 list-disc space-y-1 pl-6">{children}</ul>,
  ol: ({ children }) => <ol className="my-3 list-decimal space-y-1 pl-6">{children}</ol>,
  li: ({ children }) => <li className="leading-relaxed">{children}</li>,
  a: ({ href, children }) => <GuideLink href={href}>{children}</GuideLink>,
  blockquote: ({ children }) => (
    <blockquote className="my-4 border-l-4 border-muted-foreground/30 pl-4 text-muted-foreground">
      {children}
    </blockquote>
  ),
  table: ({ children }) => (
    <div className="my-4 overflow-x-auto">
      <table className="w-full border-collapse text-sm">{children}</table>
    </div>
  ),
  th: ({ children }) => <th className="border bg-muted px-3 py-2 text-left font-medium">{children}</th>,
  td: ({ children }) => <td className="border px-3 py-2 align-top">{children}</td>,
  pre: ({ children }) => (
    <pre className="my-4 overflow-x-auto rounded-md bg-muted p-3 text-sm">{children}</pre>
  ),
  code: ({ children, className }) => (
    <code className={className ?? 'rounded bg-muted px-1 py-0.5 text-sm'}>{children}</code>
  ),
  // Screenshots are out of scope; images are not rendered.
  img: () => null,
};

/** Renders a guide chapter. Raw HTML is not enabled (no rehype-raw). */
export function HelpChapter({ markdown }: { markdown: string }) {
  return (
    <article className="max-w-3xl">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {markdown}
      </ReactMarkdown>
    </article>
  );
}
