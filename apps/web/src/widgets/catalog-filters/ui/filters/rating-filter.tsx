'use client';

import { Star } from 'lucide-react';

interface RatingFilterProps {
  ratingMin: number | null;
  onRatingChange: (rating: number | null) => void;
}

export function RatingFilter({ ratingMin, onRatingChange }: RatingFilterProps) {
  const options = [
    { value: null, label: 'Любой' },
    { value: 5, label: '5 звезд' },
    { value: 4, label: 'от 4 звезд' },
    { value: 3, label: 'от 3 звезд' },
    { value: 2, label: 'от 2 звезд' },
    { value: 1, label: 'от 1 звезды' },
  ];

  return (
    <div className="space-y-2">
      {options.map((option) => (
        <button
          key={option.value ?? 'any'}
          onClick={() => onRatingChange(option.value)}
          className={`flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm transition-colors ${
            ratingMin === option.value
              ? 'bg-primary text-primary-foreground'
              : 'hover:bg-accent'
          }`}
        >
          {option.value !== null && (
            <Star className="h-4 w-4 fill-current" />
          )}
          <span>{option.label}</span>
        </button>
      ))}
    </div>
  );
}
