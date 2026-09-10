'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { SearchInputForm } from '@/shared/ui';

/**
 * Compact inline header search. Reuses `SearchInputForm` (the same
 * icon+input+form markup/submit behavior as the catalog `SearchBar`)
 * but navigates to `/books?search=` instead of updating in-place filters,
 * since the header isn't always rendered on `/books`.
 */
export function HeaderSearch() {
  const router = useRouter();
  const [value, setValue] = useState('');

  const handleSubmit = (submitted: string) => {
    const trimmed = submitted.trim();
    router.push(
      trimmed ? (`/books?search=${encodeURIComponent(trimmed)}` as any) : ('/books' as any)
    );
  };

  return (
    <SearchInputForm
      value={value}
      onChange={setValue}
      onSubmit={handleSubmit}
      placeholder="Поиск книг..."
      className="hidden max-w-xs md:block"
      inputClassName="h-9 border-transparent bg-background text-foreground placeholder:text-muted-foreground focus-visible:ring-primary"
    />
  );
}
