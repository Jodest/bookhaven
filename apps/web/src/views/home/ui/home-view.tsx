import { BookRow } from '@/entities/book';
import { collectionsApi } from '@/entities/curated-collection';
import { genresApi } from '@/entities/genre';
import { homeApi } from '@/entities/home';
import { PickRandomBook } from '@/features/pick-random-book';
import { BookOfTheDay } from '@/widgets/book-of-the-day';
import { CuratedCollectionsRow } from '@/widgets/curated-collections-row';
import { LiteraryCalendar } from '@/widgets/literary-calendar';
import {
  ChallengeProgress,
  PersonalHome,
  PersonalRecommendations,
  WantToReadPick,
} from '@/widgets/personal-home';
import { RecentReviews } from '@/widgets/recent-reviews';

/**
 * Home page (see docs/features/home-page-content.md). No hero. At lg+ the page
 * is two columns: a main column (flex-1: «Продолжить чтение», «Книга дня»,
 * «Из вашего «Хочу прочитать»», «Вам может понравиться», «Новое в каталоге»,
 * «Высоко оценённые», «Подборки», «Свежие рецензии») and a ~340px sidebar
 * («Читательский вызов», «Что почитать?», «Литературный календарь»). Below lg
 * it is one column: «Продолжить чтение», «Книга дня», «Хочу прочитать», the
 * sidebar blocks, then the remaining rows (the main column is `contents` and
 * its two groups are reordered with `order-*`). Public blocks are
 * server-rendered from GET /home; if it fails, the personal blocks, «Что
 * почитать?» and «Подборки» still render. «Подборки» comes from
 * GET /collections?limit=6 independently of /home. Personal blocks (logged-in
 * only) are client-side widgets over GET /home/me. Empty blocks render
 * nothing (their wrapper groups collapse via `empty:hidden`).
 */
export async function HomeView() {
  const [home, { items: genres }, collections] = await Promise.all([
    homeApi.get().catch(() => null),
    genresApi.list().catch(() => ({ items: [] as { id: string; name: string }[] })),
    collectionsApi.list({ limit: 6 }).catch(() => null),
  ]);

  const bookOfTheDay = home?.bookOfTheDay ?? null;
  const calendar =
    home?.calendar && (home.calendar.events.length > 0 || home.calendar.anniversaries.length > 0)
      ? home.calendar
      : null;

  // Skip empty collections, show the 3 newest.
  const homeCollections = (collections?.items ?? []).filter((c) => c.shown_count > 0).slice(0, 3);

  return (
    <main className="container mx-auto px-4 py-8 sm:px-6 md:py-10 lg:px-8">
      <div className="flex flex-col gap-12 lg:flex-row lg:items-start">
        <div className="contents lg:flex lg:min-w-0 lg:flex-1 lg:flex-col lg:gap-12">
          <div className="order-1 flex min-w-0 flex-col gap-12 empty:hidden lg:order-none">
            <PersonalHome />
            <BookOfTheDay data={bookOfTheDay} />
            <WantToReadPick />
          </div>

          <div className="order-3 flex min-w-0 flex-col gap-12 empty:hidden lg:order-none">
            <PersonalRecommendations />
            {home && (
              <>
                <BookRow
                  title="Новое в каталоге"
                  books={home.newBooks}
                  href="/books?sort=new"
                  linkLabel="Весь каталог"
                />
                <BookRow
                  title="Высоко оценённые"
                  books={home.topRated}
                  href="/books?sort=rating"
                  linkLabel="Смотреть все"
                />
              </>
            )}
            <CuratedCollectionsRow collections={homeCollections} />
            {home && <RecentReviews reviews={home.recentReviews} />}
          </div>
        </div>

        <aside
          aria-label="Дополнительно"
          className="order-2 grid w-full grid-cols-1 gap-6 lg:order-none lg:w-[340px] lg:shrink-0"
        >
          <ChallengeProgress />
          <PickRandomBook genres={genres.map((g) => ({ id: g.id, name: g.name }))} />
          <LiteraryCalendar calendar={calendar} />
        </aside>
      </div>
    </main>
  );
}
