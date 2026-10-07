interface PlaceholderPageProps {
  title: string;
  description?: string;
}

/** Presentational stub for sections that are planned but not implemented yet. */
export function PlaceholderPage({ title, description = 'Раздел в разработке.' }: PlaceholderPageProps) {
  return (
    <div className="space-y-2">
      <h1 className="text-3xl font-bold tracking-tight">{title}</h1>
      <p className="text-muted-foreground">{description}</p>
    </div>
  );
}
