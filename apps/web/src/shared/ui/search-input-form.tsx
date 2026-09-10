'use client';

import { Search } from 'lucide-react';
import { Input, cn } from '@bookhaven/ui';

interface SearchInputFormProps {
  value: string;
  onChange: (value: string) => void;
  onSubmit: (value: string) => void;
  placeholder?: string;
  className?: string;
  inputClassName?: string;
}

/**
 * Shared submit-on-enter search input (icon + text field + form).
 * Lives in `shared/ui` (not `widgets/catalog-filters`) so both
 * `widgets/catalog-filters`'s `SearchBar` (debounced, in-place filtering)
 * and `widgets/header`'s inline search (navigates to `/books?search=`) can
 * reuse the exact same markup/submit behavior without a disallowed
 * sibling-widget import.
 */
export function SearchInputForm({
  value,
  onChange,
  onSubmit,
  placeholder = 'Поиск по названию или ISBN...',
  className,
  inputClassName,
}: SearchInputFormProps) {
  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit(value);
  };

  return (
    <form onSubmit={handleSubmit} className={cn('relative w-full max-w-xl', className)}>
      <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        type="text"
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={cn('pl-10', inputClassName)}
      />
    </form>
  );
}
