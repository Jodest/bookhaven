'use client';

import { useEffect, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { SearchInputForm } from '@/shared/ui';

interface SearchBarProps {
  onSearch: (value: string) => void;
}

export function SearchBar({ onSearch }: SearchBarProps) {
  const searchParams = useSearchParams();
  const [value, setValue] = useState(searchParams.get('search') || '');

  // Update local state when URL changes
  useEffect(() => {
    setValue(searchParams.get('search') || '');
  }, [searchParams]);

  // Debounced search
  useEffect(() => {
    const timer = setTimeout(() => {
      onSearch(value);
    }, 300);
    return () => clearTimeout(timer);
  }, [value, onSearch]);

  return <SearchInputForm value={value} onChange={setValue} onSubmit={onSearch} />;
}
