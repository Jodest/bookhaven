import { pluralRu } from '@/shared/lib/plural-ru';
import { Star } from 'lucide-react';

interface RatingSummaryProps {
  rating: number;
  ratingsCount: number;
  reviewsCount: number;
}

/** One-line rating summary: "★ 4.3 · 12 оценок · 5 рецензий". */
export function RatingSummary({ rating, ratingsCount, reviewsCount }: RatingSummaryProps) {
  return (
    <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
      {ratingsCount > 0 ? (
        <>
          <span className="inline-flex items-center gap-1 font-semibold text-foreground">
            <Star className="h-4 w-4 fill-yellow-400 text-yellow-400" aria-hidden="true" />
            {rating.toFixed(1)}
          </span>
          <span aria-hidden="true">·</span>
          <span>
            {ratingsCount} {pluralRu(ratingsCount, ['оценка', 'оценки', 'оценок'])}
          </span>
        </>
      ) : (
        <span>Пока нет оценок</span>
      )}
      <span aria-hidden="true">·</span>
      <a href="#reviews" className="text-primary hover:underline">
        {reviewsCount > 0
          ? `${reviewsCount} ${pluralRu(reviewsCount, ['рецензия', 'рецензии', 'рецензий'])}`
          : 'Написать рецензию'}
      </a>
    </p>
  );
}
