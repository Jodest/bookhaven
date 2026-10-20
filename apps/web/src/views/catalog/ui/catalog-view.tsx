'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import type { Route } from 'next';
import Link from 'next/link';
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
import { Button, Pagination } from '@bookhaven/ui';
import { buildCatalogPageHref } from '../lib/page-href';

const PAGE_SIZE = 24;

type Status = 'initial' | 'replacing' | 'idle';

function parseNumberParam(value: string | null): number | null {
  if (value === null) return null;
  const num = parseInt(value, 10);
  return isNaN(num) ? null : num;
}

function parsePage(value: string | null): number {
  const p = parseNumberParam(value);
  return p && p > 0 ? p : 1;
}

// Query for one page, built from the filter part of the URL (`page` excluded).
function buildListQuery(filterKey: string, page: number) {
  const params = new URLSearchParams(filterKey);
  const genreIds = params.getAll('genre_id');
  return {
    page,
    limit: PAGE_SIZE,
    search: params.get('search') || undefined,
    genre_id: genreIds.length > 0 ? genreIds : undefined,
    author_id: params.get('author_id') || undefined,
    rating_min: parseNumberParam(params.get('rating_min')) || undefined,
    language: params.get('language') || undefined,
    year_from: parseNumberParam(params.get('year_from')) || undefined,
    year_to: parseNumberParam(params.get('year_to')) || undefined,
    has_adaptation: params.get('has_adaptation') === 'true' ? true : undefined,
    sort: params.get('sort') || 'rating',
  };
}

export function CatalogView() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();

  const [books, setBooks] = useState<Book[]>([]);
  const [total, setTotal] = useState(0);
  const [startPage, setStartPage] = useState(() => parsePage(searchParams.get('page')));
  const [endPage, setEndPage] = useState(() => parsePage(searchParams.get('page')));
  const [status, setStatus] = useState<Status>('initial');
  const [loadingMore, setLoadingMore] = useState(false);
  const [loadMoreError, setLoadMoreError] = useState(false);
  const gridRef = useRef<HTMLDivElement>(null);
  const requestIdRef = useRef(0);
  const skipNextUrlSyncRef = useRef(false);
  const endPageRef = useRef(endPage);
  const booksCountRef = useRef(0);
  const lastFilterKeyRef = useRef<string | null>(null);
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
  const [hasAdaptation, setHasAdaptation] = useState(() => searchParams.get('has_adaptation') === 'true');
  const [sort, setSort] = useState(searchParams.get('sort') || 'rating');

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
    setHasAdaptation(searchParams.get('has_adaptation') === 'true');
    setSort(searchParams.get('sort') || 'rating');
  }, [searchParams]);

  // Everything except `page` identifies the filter set; `urlPage` is the page.
  const filterParams = new URLSearchParams(searchParams.toString());
  filterParams.delete('page');
  const filterKey = filterParams.toString();
  const urlPage = parsePage(searchParams.get('page'));

  useEffect(() => {
    const sameFilters = lastFilterKeyRef.current === filterKey;
    lastFilterKeyRef.current = filterKey;
    const skip = skipNextUrlSyncRef.current;
    skipNextUrlSyncRef.current = false;
    // "Show more" already loaded this page and only rewrote the URL.
    if (skip && sameFilters && urlPage === endPageRef.current) return;

    const requestId = ++requestIdRef.current;
    setLoadingMore(false);
    setLoadMoreError(false);
    setStatus(booksCountRef.current === 0 ? 'initial' : 'replacing');

    booksApi
      .list(buildListQuery(filterKey, urlPage))
      .then((result) => {
        if (requestId !== requestIdRef.current) return;
        booksCountRef.current = result.items.length;
        endPageRef.current = urlPage;
        setBooks(result.items);
        setTotal(result.total);
        setStartPage(urlPage);
        setEndPage(urlPage);
        setStatus('idle');
      })
      .catch((err) => {
        if (requestId !== requestIdRef.current) return;
        console.error('Failed to fetch books:', err);
        booksCountRef.current = 0;
        setBooks([]);
        setTotal(0);
        setStatus('idle');
      });
  }, [filterKey, urlPage]);

  const loadMore = useCallback(async () => {
    const nextPage = endPageRef.current + 1;
    const requestId = ++requestIdRef.current;
    setLoadingMore(true);
    setLoadMoreError(false);
    try {
      const result = await booksApi.list(buildListQuery(filterKey, nextPage));
      if (requestId !== requestIdRef.current) return;
      setBooks((prev) => {
        const seen = new Set(prev.map((b) => b.id));
        const merged = [...prev, ...result.items.filter((b) => !seen.has(b.id))];
        booksCountRef.current = merged.length;
        return merged;
      });
      setTotal(result.total);
      setEndPage(nextPage);
      endPageRef.current = nextPage;
      skipNextUrlSyncRef.current = true;
      window.history.replaceState(null, '', buildCatalogPageHref(pathname, searchParams, nextPage));
    } catch (err) {
      if (requestId !== requestIdRef.current) return;
      console.error('Failed to load more books:', err);
      setLoadMoreError(true);
    } finally {
      if (requestId === requestIdRef.current) setLoadingMore(false);
    }
  }, [filterKey, pathname, searchParams]);

  const scrollToGrid = useCallback(() => {
    const reduce =
      typeof window.matchMedia === 'function' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    gridRef.current?.scrollIntoView({ block: 'start', behavior: reduce ? 'auto' : 'smooth' });
  }, []);

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
      updateURL({ search: value || undefined, page: undefined });
    },
    [updateURL]
  );

  const handleGenreChange = useCallback(
    (newGenreIds: string[]) => {
      setGenreIds(newGenreIds);
      updateURL({ genre_id: newGenreIds.length > 0 ? newGenreIds : undefined, page: undefined });
    },
    [updateURL]
  );

  const handleAuthorSelect = useCallback(
    (newAuthorId: string | null, newAuthorName: string | null) => {
      setAuthorId(newAuthorId);
      setAuthorName(newAuthorName);
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
      updateURL({ rating_min: newRating || undefined, page: undefined });
    },
    [updateURL]
  );

  const handleLanguageChange = useCallback(
    (newLanguage: string | null) => {
      setLanguage(newLanguage);
      updateURL({ language: newLanguage || undefined, page: undefined });
    },
    [updateURL]
  );

  const handleYearFromChange = useCallback(
    (year: number | null) => {
      setYearFrom(year);
      updateURL({ year_from: year || undefined, page: undefined });
    },
    [updateURL]
  );

  const handleYearToChange = useCallback(
    (year: number | null) => {
      setYearTo(year);
      updateURL({ year_to: year || undefined, page: undefined });
    },
    [updateURL]
  );

  const handleHasAdaptationChange = useCallback(
    (value: boolean) => {
      setHasAdaptation(value);
      updateURL({ has_adaptation: value ? 'true' : undefined, page: undefined });
    },
    [updateURL]
  );

  const handleSortChange = useCallback(
    (newSort: string) => {
      setSort(newSort);
      updateURL({ sort: newSort, page: undefined });
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
    setHasAdaptation(false);
    setSort('rating');
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

  if (hasAdaptation) {
    chips.push({
      key: 'has_adaptation',
      label: 'Есть экранизация',
      onRemove: () => handleHasAdaptationChange(false),
    });
  }

  const totalPages = Math.ceil(total / PAGE_SIZE);
  const remaining = Math.min(PAGE_SIZE, total - endPage * PAGE_SIZE);

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
              hasAdaptation={hasAdaptation}
              onHasAdaptationChange={handleHasAdaptationChange}
            />
          </div>
        </aside>

        <div className="flex-1">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <ActiveFilters chips={chips} onClearAll={clearAllFilters} />
            <SortSelector sort={sort} onSortChange={handleSortChange} />
          </div>

          {status === 'initial' ? (
            <div className="grid grid-cols-2 gap-6 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
              {Array.from({ length: 8 }).map((_, i) => (
                <div
                  key={i}
                  className="h-[400px] animate-pulse rounded-xl bg-muted"
                />
              ))}
            </div>
          ) : books.length === 0 ? (
            total > 0 ? (
              <div className="py-12 text-center">
                <p className="mb-3 text-muted-foreground">На этой странице книг нет</p>
                <Link
                  href={buildCatalogPageHref(pathname, searchParams, totalPages) as Route}
                  scroll={false}
                  className="text-primary underline underline-offset-4"
                >
                  Перейти на последнюю страницу
                </Link>
              </div>
            ) : (
              <EmptyState />
            )
          ) : (
            <>
              <div
                ref={gridRef}
                aria-busy={status === 'replacing'}
                className={`scroll-mt-6 ${
                  status === 'replacing'
                    ? 'pointer-events-none opacity-50 transition-opacity'
                    : 'transition-opacity'
                }`}
              >
                <div className="grid grid-cols-2 gap-6 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
                  {books.map((book) => (
                    <BookCard key={book.id} book={book} />
                  ))}
                </div>
              </div>

              {endPage < totalPages && (
                <div className="mt-8 flex flex-col items-stretch gap-2 sm:items-center">
                  <Button
                    variant="outline"
                    className="w-full sm:w-auto"
                    disabled={loadingMore || status !== 'idle'}
                    aria-busy={loadingMore}
                    onClick={loadMore}
                  >
                    {loadingMore ? 'Загрузка…' : `Показать ещё ${remaining}`}
                  </Button>
                  {loadMoreError && (
                    <p role="alert" className="text-center text-sm text-destructive">
                      Не удалось загрузить книги
                    </p>
                  )}
                </div>
              )}

              {totalPages > 1 && (
                <Pagination
                  className="mt-6"
                  page={endPage}
                  prevPage={startPage - 1}
                  nextPage={endPage + 1}
                  totalPages={totalPages}
                  getHref={(p) => buildCatalogPageHref(pathname, searchParams, p)}
                  onPageChange={scrollToGrid}
                  renderLink={({ href, ...rest }) => (
                    <Link href={href as Route} scroll={false} {...rest} />
                  )}
                />
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
              hasAdaptation={hasAdaptation}
              onHasAdaptationChange={handleHasAdaptationChange}
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
