'use client';

import { Select } from '@bookhaven/ui';

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
    <Select
      options={sortOptions}
      value={sort}
      onChange={(e) => onSortChange(e.target.value)}
      className="w-full max-w-[200px]"
    />
  );
}
