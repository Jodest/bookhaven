import Link from 'next/link';

interface AuthorNameLinkProps {
  author: { name: string; slug: string | null };
  /** Applied to both the link and the plain-text fallback. */
  className?: string;
  /** Extra classes for the link only (e.g. hover underline, link color). */
  linkClassName?: string;
}

/**
 * Author name as a link to the author page, or plain text when the author
 * is not published (`slug === null` in public book payloads, see
 * docs/features/author-status.md) — never a link to `/authors/null`.
 */
export function AuthorNameLink({ author, className, linkClassName }: AuthorNameLinkProps) {
  if (!author.slug) {
    return <span className={className}>{author.name}</span>;
  }
  const classes = [className, linkClassName].filter(Boolean).join(' ') || undefined;
  return (
    <Link href={`/authors/${author.slug}`} className={classes}>
      {author.name}
    </Link>
  );
}
