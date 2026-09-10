'use client';

import { cn } from '@bookhaven/ui';

interface SortSelectorProps {
  sort: string;
  onSortChange: (sort: string) => void;
}

const sortOptions = [
  { value: 'rating', label: 'По рейтингу' },
  { value: 'new', label: 'По дате' },
  { value: 'title', label: 'По названию' },
];

export function SortSelector({ sort, onSortChange }: SortSelectorProps) {
  return (
    <div
      role="radiogroup"
      aria-label="Сортировка"
      className="inline-flex flex-wrap gap-1 rounded-full border bg-card p-1"
    >
      {sortOptions.map((option) => (
        <button
          key={option.value}
          type="button"
          role="radio"
          aria-checked={sort === option.value}
          onClick={() => onSortChange(option.value)}
          className={cn(
            'rounded-full px-3 py-1.5 text-sm font-medium transition-colors',
            sort === option.value
              ? 'bg-primary text-primary-foreground'
              : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
