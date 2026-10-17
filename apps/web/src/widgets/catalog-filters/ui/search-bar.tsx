'use client';

import { useEffect, useRef, useState } from 'react';
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

  // Keep latest callback in a ref so its identity changes don't retrigger the debounce
  const onSearchRef = useRef(onSearch);
  useEffect(() => {
    onSearchRef.current = onSearch;
  });

  const urlSearch = searchParams.get('search') || '';
  const urlSearchRef = useRef(urlSearch);
  urlSearchRef.current = urlSearch;

  // Debounced search: only when the user changed the text (not when synced from URL)
  useEffect(() => {
    if (value === urlSearchRef.current) return;
    const timer = setTimeout(() => {
      onSearchRef.current(value);
    }, 300);
    return () => clearTimeout(timer);
  }, [value]);

  return <SearchInputForm value={value} onChange={setValue} onSubmit={onSearch} />;
}
