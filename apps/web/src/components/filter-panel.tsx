'use client';

import { GenreFilter } from '@/components/filters/genre-filter';
import { AuthorFilter } from '@/components/filters/author-filter';
import { RatingFilter } from '@/components/filters/rating-filter';
import { LanguageFilter } from '@/components/filters/language-filter';
import { YearFilter } from '@/components/filters/year-filter';

interface FilterPanelProps {
  genreIds: string[];
  onGenreChange: (genreIds: string[]) => void;
  authorId: string | null;
  authorName: string | null;
  onAuthorSelect: (authorId: string | null, authorName: string | null) => void;
  ratingMin: number | null;
  onRatingChange: (rating: number | null) => void;
  language: string | null;
  onLanguageChange: (language: string | null) => void;
  yearFrom: number | null;
  yearTo: number | null;
  onYearFromChange: (year: number | null) => void;
  onYearToChange: (year: number | null) => void;
}

export function FilterPanel({
  genreIds,
  onGenreChange,
  authorId,
  authorName,
  onAuthorSelect,
  ratingMin,
  onRatingChange,
  language,
  onLanguageChange,
  yearFrom,
  yearTo,
  onYearFromChange,
  onYearToChange,
}: FilterPanelProps) {
  return (
    <div className="space-y-6">
      <div>
        <h3 className="mb-3 text-sm font-medium">Жанры</h3>
        <GenreFilter selectedGenreIds={genreIds} onGenreChange={onGenreChange} />
      </div>

      <div>
        <h3 className="mb-3 text-sm font-medium">Автор</h3>
        <AuthorFilter
          selectedAuthorId={authorId}
          selectedAuthorName={authorName}
          onAuthorSelect={onAuthorSelect}
        />
      </div>

      <div>
        <h3 className="mb-3 text-sm font-medium">Рейтинг</h3>
        <RatingFilter ratingMin={ratingMin} onRatingChange={onRatingChange} />
      </div>

      <div>
        <h3 className="mb-3 text-sm font-medium">Язык</h3>
        <LanguageFilter
          selectedLanguage={language}
          onLanguageChange={onLanguageChange}
        />
      </div>

      <div>
        <h3 className="mb-3 text-sm font-medium">Год издания</h3>
        <YearFilter
          yearFrom={yearFrom}
          yearTo={yearTo}
          onYearFromChange={onYearFromChange}
          onYearToChange={onYearToChange}
        />
      </div>
    </div>
  );
}
