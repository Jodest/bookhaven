const SKELETON_COUNT = 4;

interface RowSkeletonProps {
  title: string;
  className?: string;
}

/** Loading placeholder for a titled book row (same markup as SimilarBooks). */
export function RowSkeleton({ title, className }: RowSkeletonProps) {
  return (
    <section aria-busy="true" aria-label={title} className={className}>
      <h2 className="font-display text-xl font-semibold md:text-2xl">{title}</h2>
      <ul className="mt-4 flex gap-4 overflow-hidden pb-2">
        {Array.from({ length: SKELETON_COUNT }, (_, i) => (
          <li key={i} className="w-40 shrink-0 sm:w-44" data-testid="personal-home-skeleton">
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
