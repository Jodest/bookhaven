'use client';

import { X } from 'lucide-react';

export interface FilterChip {
  key: string;
  label: string;
  onRemove: () => void;
}

interface ActiveFiltersProps {
  chips: FilterChip[];
  onClearAll: () => void;
}

export function ActiveFilters({ chips, onClearAll }: ActiveFiltersProps) {
  if (chips.length === 0) {
    return null;
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      {chips.map((chip) => (
        <span
          key={chip.key}
          className="inline-flex items-center gap-1 rounded-full bg-secondary px-3 py-1 text-sm"
        >
          {chip.label}
          <button
            onClick={chip.onRemove}
            className="ml-1 rounded-full hover:bg-muted-foreground/20"
            aria-label={`Remove ${chip.label}`}
          >
            <X className="h-3 w-3" />
          </button>
        </span>
      ))}
      <button
        onClick={onClearAll}
        className="text-sm text-muted-foreground hover:text-foreground"
      >
        Сбросить все
      </button>
    </div>
  );
}
