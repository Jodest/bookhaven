const SKELETON_COUNT = 5;

// Same one-row column counts as BookRow (2 rows of 2 on phones).
const SKELETON_VISIBILITY = ['', '', '', 'sm:hidden md:block lg:hidden xl:block', 'hidden 2xl:block'];

interface RowSkeletonProps {
  title: string;
  className?: string;
}

/** Loading placeholder for a titled book row (same grid as BookRow). */
export function RowSkeleton({ title, className }: RowSkeletonProps) {
  return (
    <section aria-busy="true" aria-label={title} className={className}>
      <h2 className="font-display text-xl font-semibold md:text-2xl">{title}</h2>
      <ul className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5">
        {Array.from({ length: SKELETON_COUNT }, (_, i) => (
          <li key={i} className={`min-w-0 ${SKELETON_VISIBILITY[i] ?? ''}`} data-testid="personal-home-skeleton">
            <div className="aspect-[2/3] animate-pulse rounded-xl bg-muted" />
            <div className="mt-3 h-4 w-3/4 animate-pulse rounded bg-muted" />
            <div className="mt-2 h-3 w-1/2 animate-pulse rounded bg-muted" />
          </li>
        ))}
      </ul>
    </section>
  );
}

/** Loading placeholder for a personal card (challenge, want-to-read pick). */
export function CardSkeleton({ label }: { label: string }) {
  return (
    <div
      aria-busy="true"
      aria-label={label}
      data-testid="personal-home-skeleton"
      className="h-36 animate-pulse rounded-2xl border bg-muted"
    />
  );
}
