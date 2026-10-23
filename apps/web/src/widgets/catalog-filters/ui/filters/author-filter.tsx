'use client';

import { useState, useEffect, useRef } from 'react';
import { authorsApi } from '@/entities/author';
import { Input } from '@bookhaven/ui';
import { Search } from 'lucide-react';

interface Author {
  id: string;
  name: string;
  slug: string | null;
}

interface AuthorFilterProps {
  selectedAuthorId: string | null;
  selectedAuthorName: string | null;
  onAuthorSelect: (authorId: string | null, authorName: string | null) => void;
}

export function AuthorFilter({
  selectedAuthorId,
  selectedAuthorName,
  onAuthorSelect,
}: AuthorFilterProps) {
  const [search, setSearch] = useState('');
  const [results, setResults] = useState<Author[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const debounceRef = useRef<NodeJS.Timeout | undefined>(undefined);

  useEffect(() => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
    }

    if (!search) {
      setResults([]);
      return;
    }

    debounceRef.current = setTimeout(async () => {
      try {
        const result = await authorsApi.list({ search, limit: 10 });
        setResults(result.items);
        setIsOpen(true);
      } catch (err) {
        console.error('Failed to search authors:', err);
        setResults([]);
      }
    }, 300);

    return () => {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current);
      }
    };
  }, [search]);

  const handleSelect = (author: Author) => {
    onAuthorSelect(author.id, author.name);
    setSearch('');
    setResults([]);
    setIsOpen(false);
  };

  const handleClear = () => {
    onAuthorSelect(null, null);
    setSearch('');
    setResults([]);
    setIsOpen(false);
  };

  return (
    <div className="space-y-2">
      {selectedAuthorId && selectedAuthorName ? (
        <div className="flex items-center gap-2 rounded-md border px-3 py-2">
          <span className="flex-1 text-sm">{selectedAuthorName}</span>
          <button
            onClick={handleClear}
            className="text-muted-foreground hover:text-foreground"
          >
            ×
          </button>
        </div>
      ) : (
        <>
          <div className="relative">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="text"
              placeholder="Поиск автора..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-10"
            />
          </div>
          {isOpen && results.length > 0 && (
            <ul className="absolute z-10 mt-1 max-h-48 w-full overflow-auto rounded-md border bg-popover shadow-md">
              {results.map((author) => (
                <li
                  key={author.id}
                  onClick={() => handleSelect(author)}
                  className="cursor-pointer px-3 py-2 text-sm hover:bg-accent"
                >
                  {author.name}
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </div>
  );
}
