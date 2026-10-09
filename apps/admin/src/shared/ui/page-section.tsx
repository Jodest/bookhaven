import { useId, type ReactNode } from 'react';

interface PageSectionProps {
  title: string;
  description?: string;
  children: ReactNode;
}

/**
 * A bordered, titled block of a page (h2 + optional one-line muted
 * description). Used to stack the sections of the Import tabs
 * (docs/features/import-by-entity.md).
 */
export function PageSection({ title, description, children }: PageSectionProps) {
  const headingId = useId();

  return (
    <section aria-labelledby={headingId} className="space-y-4 rounded-lg border p-6">
      <div className="space-y-1">
        <h2 id={headingId} className="text-xl font-semibold tracking-tight">
          {title}
        </h2>
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
      </div>
      {children}
    </section>
  );
}
