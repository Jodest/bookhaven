'use client';

import { Input } from '@/components/ui/input';

interface YearFilterProps {
  yearFrom: number | null;
  yearTo: number | null;
  onYearFromChange: (year: number | null) => void;
  onYearToChange: (year: number | null) => void;
}

export function YearFilter({
  yearFrom,
  yearTo,
  onYearFromChange,
  onYearToChange,
}: YearFilterProps) {
  const handleYearFrom = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    if (value === '') {
      onYearFromChange(null);
    } else {
      const num = parseInt(value, 10);
      if (!isNaN(num) && num >= 1000 && num <= 2099) {
        onYearFromChange(num);
      }
    }
  };

  const handleYearTo = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    if (value === '') {
      onYearToChange(null);
    } else {
      const num = parseInt(value, 10);
      if (!isNaN(num) && num >= 1000 && num <= 2099) {
        onYearToChange(num);
      }
    }
  };

  return (
    <div className="flex items-center gap-2">
      <Input
        type="number"
        placeholder="От"
        min={1000}
        max={2099}
        value={yearFrom ?? ''}
        onChange={handleYearFrom}
        className="w-24"
      />
      <span className="text-muted-foreground">—</span>
      <Input
        type="number"
        placeholder="До"
        min={1000}
        max={2099}
        value={yearTo ?? ''}
        onChange={handleYearTo}
        className="w-24"
      />
    </div>
  );
}
