'use client';

import { useState, useEffect, useCallback } from 'react';
import { useRouter, usePathname, useSearchParams } from 'next/navigation';
import { booksApi, BookCard } from '@/entities/book';
import { Book } from '@bookhaven/api-client';
import {
  SearchBar,
  FilterPanel,
  ActiveFilters,
  SortSelector,
} from '@/widgets/catalog-filters';
import type { FilterChip } from '@/widgets/catalog-filters';
import { EmptyState } from '@/shared/ui';
import { SlidersHorizontal } from 'lucide-react';
import { Button } from '@bookhaven/ui';

function parseNumberParam(value: string | null): number | null {
  if (value === null) return null;
  const num = parseInt(value, 10);
  return isNaN(num) ? null : num;
}

export function CatalogView() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();

  const [books, setBooks] = useState<Book[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [showMobileFilters, setShowMobileFilters] = useState(false);

  // Filter state - initialize from URL
  const [search, setSearch] = useState(searchParams.get('search') || '');
  const [genreIds, setGenreIds] = useState<string[]>(() => {
    return searchParams.getAll('genre_id');
  });
  const [authorId, setAuthorId] = useState<string | null>(() => {
    return searchParams.get('author_id');
  });
  const [authorName, setAuthorName] = useState<string | null>(() => {
    return searchParams.get('author_name');
  });
  const [ratingMin, setRatingMin] = useState<number | null>(() => {
    return parseNumberParam(searchParams.get('rating_min'));
  });
  const [language, setLanguage] = useState<string | null>(() => {
    return searchParams.get('language');
  });
  const [yearFrom, setYearFrom] = useState<number | null>(() => {
    return parseNumberParam(searchParams.get('year_from'));
  });
  const [yearTo, setYearTo] = useState<number | null>(() => {
    return parseNumberParam(searchParams.get('year_to'));
  });
  const [sort, setSort] = useState(searchParams.get('sort') || 'rating');
  const [page, setPage] = useState(() => {
    const p = parseNumberParam(searchParams.get('page'));
    return p && p > 0 ? p : 1;
  });

  // Sync state when URL changes (e.g., browser back/forward)
  useEffect(() => {
    setSearch(searchParams.get('search') || '');
    setGenreIds(searchParams.getAll('genre_id'));
    setAuthorId(searchParams.get('author_id'));
    setAuthorName(searchParams.get('author_name'));
    setRatingMin(parseNumberParam(searchParams.get('rating_min')));
    setLanguage(searchParams.get('language'));
    setYearFrom(parseNumberParam(searchParams.get('year_from')));
    setYearTo(parseNumberParam(searchParams.get('year_to')));
    setSort(searchParams.get('sort') || 'rating');
    const p = parseNumberParam(searchParams.get('page'));
    setPage(p && p > 0 ? p : 1);
  }, [searchParams]);

  // Fetch books when filters change
  useEffect(() => {
    async function fetchBooks() {
      setLoading(true);
      try {
        const result = await booksApi.list({
          page,
          limit: 20,
          search: search || undefined,
          genre_id: genreIds.length > 0 ? genreIds : undefined,
          author_id: authorId || undefined,
          rating_min: ratingMin || undefined,
          language: language || undefined,
          year_from: yearFrom || undefined,
          year_to: yearTo || undefined,
          sort,
        });
        setBooks(result.items);
        setTotal(result.total);
      } catch (err) {
        console.error('Failed to fetch books:', err);
        setBooks([]);
        setTotal(0);
      } finally {
        setLoading(false);
      }
    }
    fetchBooks();
  }, [page, search, genreIds, authorId, ratingMin, language, yearFrom, yearTo, sort]);

  // Update URL when filters change
  const updateURL = useCallback(
    (updates: Record<string, string | string[] | number | undefined>) => {
      const newParams = new URLSearchParams(searchParams.toString());

      // Apply updates
      for (const [key, value] of Object.entries(updates)) {
        newParams.delete(key);
        if (value === undefined || value === null || value === '') continue;
        if (Array.isArray(value)) {
          for (const v of value) {
            if (v !== '') newParams.append(key, String(v));
          }
        } else {
          newParams.set(key, String(value));
        }
      }

      const queryString = newParams.toString();
      router.push(`${pathname}${queryString ? `?${queryString}` : ''}` as any, {
        scroll: false,
      });
    },
    [pathname, router, searchParams]
  );

  const handleSearch = useCallback(
    (value: string) => {
      setSearch(value);
      setPage(1);
      updateURL({ search: value || undefined, page: undefined });
    },
    [updateURL]
  );

  const handleGenreChange = useCallback(
    (newGenreIds: string[]) => {
      setGenreIds(newGenreIds);
      setPage(1);
      updateURL({ genre_id: newGenreIds.length > 0 ? newGenreIds : undefined, page: undefined });
    },
    [updateURL]
  );

  const handleAuthorSelect = useCallback(
    (newAuthorId: string | null, newAuthorName: string | null) => {
      setAuthorId(newAuthorId);
      setAuthorName(newAuthorName);
      setPage(1);
      updateURL({
        author_id: newAuthorId || undefined,
        author_name: newAuthorName || undefined,
        page: undefined,
      });
    },
    [updateURL]
  );

  const handleRatingChange = useCallback(
    (newRating: number | null) => {
      setRatingMin(newRating);
      setPage(1);
      updateURL({ rating_min: newRating || undefined, page: undefined });
    },
    [updateURL]
  );

  const handleLanguageChange = useCallback(
    (newLanguage: string | null) => {
      setLanguage(newLanguage);
      setPage(1);
      updateURL({ language: newLanguage || undefined, page: undefined });
    },
    [updateURL]
  );

  const handleYearFromChange = useCallback(
    (year: number | null) => {
      setYearFrom(year);
      setPage(1);
      updateURL({ year_from: year || undefined, page: undefined });
    },
    [updateURL]
  );

  const handleYearToChange = useCallback(
    (year: number | null) => {
      setYearTo(year);
      setPage(1);
      updateURL({ year_to: year || undefined, page: undefined });
    },
    [updateURL]
  );

  const handleSortChange = useCallback(
    (newSort: string) => {
      setSort(newSort);
      setPage(1);
      updateURL({ sort: newSort, page: undefined });
    },
    [updateURL]
  );

  const handlePageChange = useCallback(
    (newPage: number) => {
      setPage(newPage);
      updateURL({ page: newPage > 1 ? newPage : undefined });
    },
    [updateURL]
  );

  const clearAllFilters = useCallback(() => {
    setSearch('');
    setGenreIds([]);
    setAuthorId(null);
    setAuthorName(null);
    setRatingMin(null);
    setLanguage(null);
    setYearFrom(null);
    setYearTo(null);
    setSort('rating');
    setPage(1);
    router.push(pathname as any, { scroll: false });
  }, [pathname, router]);

  // Build active filter chips
  const chips: FilterChip[] = [];

  if (search) {
    chips.push({
      key: 'search',
      label: `Поиск: ${search}`,
      onRemove: () => handleSearch(''),
    });
  }

  if (genreIds.length > 0) {
    chips.push({
      key: 'genres',
      label: `Жанры: ${genreIds.length}`,
      onRemove: () => handleGenreChange([]),
    });
  }

  if (authorId && authorName) {
    chips.push({
      key: 'author',
      label: `Автор: ${authorName}`,
      onRemove: () => handleAuthorSelect(null, null),
    });
  }

  if (ratingMin !== null) {
    chips.push({
      key: 'rating',
      label: `Рейтинг: от ${ratingMin}`,
      onRemove: () => handleRatingChange(null),
    });
  }

  if (language) {
    chips.push({
      key: 'language',
      label: `Язык: ${language}`,
      onRemove: () => handleLanguageChange(null),
    });
  }

  if (yearFrom !== null) {
    chips.push({
      key: 'year_from',
      label: `Год от: ${yearFrom}`,
      onRemove: () => handleYearFromChange(null),
    });
  }

  if (yearTo !== null) {
    chips.push({
      key: 'year_to',
      label: `Год до: ${yearTo}`,
      onRemove: () => handleYearToChange(null),
    });
  }

  const limit = 20;
  const totalPages = Math.ceil(total / limit);

  return (
    <main className="container mx-auto px-4 py-12 sm:px-6 md:py-16 lg:px-8">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="font-display text-2xl font-semibold tracking-tight md:text-3xl">
          Каталог книг
        </h1>
        <Button
          variant="outline"
          size="sm"
          className="lg:hidden"
          onClick={() => setShowMobileFilters(true)}
        >
          <SlidersHorizontal className="mr-2 h-4 w-4" />
          Фильтры
        </Button>
      </div>

      <div className="mb-6">
        <SearchBar onSearch={handleSearch} />
      </div>

      <div className="flex gap-8">
        {/* Desktop filter panel */}
        <aside className="hidden w-64 shrink-0 lg:block">
          <div className="rounded-xl border bg-card p-4">
            <FilterPanel
              genreIds={genreIds}
              onGenreChange={handleGenreChange}
              authorId={authorId}
              authorName={authorName}
              onAuthorSelect={handleAuthorSelect}
              ratingMin={ratingMin}
              onRatingChange={handleRatingChange}
              language={language}
              onLanguageChange={handleLanguageChange}
              yearFrom={yearFrom}
              yearTo={yearTo}
              onYearFromChange={handleYearFromChange}
              onYearToChange={handleYearToChange}
            />
          </div>
        </aside>

        <div className="flex-1">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <ActiveFilters chips={chips} onClearAll={clearAllFilters} />
            <SortSelector sort={sort} onSortChange={handleSortChange} />
          </div>

          {loading ? (
            <div className="grid grid-cols-2 gap-6 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
              {Array.from({ length: 8 }).map((_, i) => (
                <div
                  key={i}
                  className="h-[400px] animate-pulse rounded-xl bg-muted"
                />
              ))}
            </div>
          ) : books.length === 0 ? (
            <EmptyState />
          ) : (
            <>
              <div className="grid grid-cols-2 gap-6 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
                {books.map((book) => (
                  <BookCard key={book.id} book={book} />
                ))}
              </div>

              {totalPages > 1 && (
                <div className="mt-8 flex justify-center gap-2">
                  {page > 1 && (
                    <button
                      onClick={() => handlePageChange(page - 1)}
                      className="rounded-md border px-4 py-2 hover:bg-accent"
                    >
                      Назад
                    </button>
                  )}
                  <span className="px-4 py-2">
                    Страница {page} из {totalPages}
                  </span>
                  {page < totalPages && (
                    <button
                      onClick={() => handlePageChange(page + 1)}
                      className="rounded-md border px-4 py-2 hover:bg-accent"
                    >
                      Вперед
                    </button>
                  )}
                </div>
              )}
            </>
          )}
        </div>
      </div>

      {/* Mobile filter drawer */}
      {showMobileFilters && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <div
            className="absolute inset-0 bg-black/50"
            onClick={() => setShowMobileFilters(false)}
          />
          <div className="absolute bottom-0 left-0 right-0 max-h-[80vh] overflow-y-auto rounded-t-xl bg-background p-6">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold">Фильтры</h2>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setShowMobileFilters(false)}
              >
                Закрыть
              </Button>
            </div>
            <FilterPanel
              genreIds={genreIds}
              onGenreChange={handleGenreChange}
              authorId={authorId}
              authorName={authorName}
              onAuthorSelect={handleAuthorSelect}
              ratingMin={ratingMin}
              onRatingChange={handleRatingChange}
              language={language}
              onLanguageChange={handleLanguageChange}
              yearFrom={yearFrom}
              yearTo={yearTo}
              onYearFromChange={handleYearFromChange}
              onYearToChange={handleYearToChange}
            />
            <Button
              className="mt-4 w-full"
              onClick={() => setShowMobileFilters(false)}
            >
              Показать результаты
            </Button>
          </div>
        </div>
      )}
    </main>
  );
}
