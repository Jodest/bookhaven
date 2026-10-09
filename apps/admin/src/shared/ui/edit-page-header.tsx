import Link from 'next/link';

interface EditPageHeaderProps {
  /** Already resolved through safeBackHref. */
  backHref: string;
  title: string;
  subtitle?: string;
}

/**
 * «← Назад» link + h1 (+ muted entity name) of the catalog edit pages
 * (docs/features/catalog-unified-lists.md).
 */
export function EditPageHeader({ backHref, title, subtitle }: EditPageHeaderProps) {
  return (
    <div className="space-y-2">
      <Link href={backHref as any} className="text-sm text-primary hover:underline">
        ← Назад
      </Link>
      <h1 className="text-3xl font-bold tracking-tight">{title}</h1>
      {subtitle && <p className="text-muted-foreground">{subtitle}</p>}
    </div>
  );
}
