import { BookRow } from '@/entities/book';
import { genresApi } from '@/entities/genre';
import { homeApi } from '@/entities/home';
import { PickRandomBook } from '@/features/pick-random-book';
import { BookOfTheDay } from '@/widgets/book-of-the-day';
import { LiteraryCalendar } from '@/widgets/literary-calendar';
import { PersonalHome, PersonalRecommendations } from '@/widgets/personal-home';
import { RecentReviews } from '@/widgets/recent-reviews';
import { Button } from '@bookhaven/ui';
import Link from 'next/link';

/**
 * Home page (see docs/features/home-page-content.md). Public blocks are
 * server-rendered from GET /home; if it fails, only the hero, genre chips
 * and the personal blocks render. Personal blocks (logged-in only) are
 * client-side widgets over GET /home/me. Empty blocks render nothing.
 */
export async function HomeView() {
  const [home, { items: genres }] = await Promise.all([
    homeApi.get().catch(() => null),
    genresApi.list().catch(() => ({ items: [] as { id: string; name: string }[] })),
  ]);

  const bookOfTheDay = home?.bookOfTheDay ?? null;
  const calendar =
    home?.calendar && (home.calendar.events.length > 0 || home.calendar.anniversaries.length > 0)
      ? home.calendar
      : null;

  return (
    <main>
      <section className="container mx-auto px-4 py-8 sm:px-6 md:py-10 lg:px-8">
        <div className="mx-auto max-w-2xl text-center">
          <h1 className="font-display text-5xl font-semibold tracking-tight md:text-6xl">
            BookHaven
          </h1>
          <p className="mt-4 text-sm text-muted-foreground md:text-base">
            Социальный каталог книг с оценками, ревью и подборками
          </p>
          <div className="mt-6 flex justify-center gap-4">
            <Button size="lg" asChild>
              <Link href="/books">Найти книгу</Link>
            </Button>
            <Button size="lg" variant="outline" asChild>
              <Link href="/shelves">Мои полки</Link>
            </Button>
          </div>
        </div>

        {genres.length > 0 && (
          <div className="mt-8 flex gap-2 overflow-x-auto pb-2">
            {genres.slice(0, 12).map((genre) => (
              <Link
                key={genre.id}
                href={`/books?genre_id=${genre.id}`}
                className="shrink-0 rounded-full border bg-card px-4 py-2 text-sm font-medium text-card-foreground shadow-sm transition hover:-translate-y-0.5 hover:shadow-card"
              >
                {genre.name}
              </Link>
            ))}
          </div>
        )}
      </section>

      <PersonalHome />

      {home && (
        <div className="container mx-auto space-y-12 px-4 pb-12 sm:px-6 md:pb-16 lg:px-8">
          {(bookOfTheDay || calendar) && (
            <div
              className={
                bookOfTheDay && calendar
                  ? 'grid grid-cols-1 items-start gap-6 lg:grid-cols-[3fr_2fr]'
                  : 'grid grid-cols-1 gap-6'
              }
            >
              <BookOfTheDay data={bookOfTheDay} />
              <LiteraryCalendar calendar={calendar} />
            </div>
          )}

          <PersonalRecommendations />

          <PickRandomBook genres={genres.map((g) => ({ id: g.id, name: g.name }))} />

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

          <RecentReviews reviews={home.recentReviews} />
        </div>
      )}

      {!home && (
        <PersonalRecommendations className="container mx-auto px-4 pb-12 sm:px-6 md:pb-16 lg:px-8" />
      )}
    </main>
  );
}
