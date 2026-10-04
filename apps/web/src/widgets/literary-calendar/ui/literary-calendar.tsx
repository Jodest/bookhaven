import { HomeCalendar, HomeCalendarEvent } from '@bookhaven/api-client';
import { formatDayMonth, formatYears } from '@/shared/lib/format-date-ru';
import Link from 'next/link';

interface LiteraryCalendarProps {
  calendar: HomeCalendar | null;
  className?: string;
}

function eventLabel(event: HomeCalendarEvent): string {
  const verb = event.type === 'birth' ? 'Родился' : 'Умер';
  return `${verb} ${formatYears(event.years)} назад`;
}

function AuthorAvatar({ name, url }: { name: string; url: string | null }) {
  return (
    <div className="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-full bg-muted text-sm font-medium">
      {url ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={url} alt="" className="h-full w-full object-cover" loading="lazy" />
      ) : (
        name.charAt(0).toUpperCase()
      )}
    </div>
  );
}

/**
 * "Литературный календарь": authors born or who died on this day (or the
 * nearest day with events within a week) plus this year's book
 * anniversaries. Renders nothing when there is no calendar.
 */
export function LiteraryCalendar({ calendar, className }: LiteraryCalendarProps) {
  if (!calendar || (calendar.events.length === 0 && calendar.anniversaries.length === 0)) {
    return null;
  }

  const hasEvents = calendar.events.length > 0;
  let heading = 'Литературный календарь';
  if (hasEvents) {
    heading = calendar.isToday
      ? `Литературный календарь · ${formatDayMonth(calendar.date)}`
      : `Скоро в календаре · ${formatDayMonth(calendar.date)}`;
  }

  return (
    <section
      aria-labelledby="literary-calendar-heading"
      className={`rounded-2xl border bg-card p-5 text-card-foreground shadow-sm md:p-6 ${className ?? ''}`}
    >
      <h2 id="literary-calendar-heading" className="font-display text-xl font-semibold md:text-2xl">
        {heading}
      </h2>

      {hasEvents && (
        <ul className="mt-4 space-y-4">
          {calendar.events.map((event) => (
            <li key={`${event.author.id}-${event.type}`} className="flex gap-3">
              <AuthorAvatar name={event.author.name} url={event.author.photoUrl} />
              <div className="min-w-0">
                <Link href={`/authors/${event.author.slug}`} className="font-medium hover:underline">
                  {event.author.name}
                </Link>
                <p className="text-sm text-muted-foreground">{eventLabel(event)}</p>
                {event.books.length > 0 && (
                  <p className="mt-1 text-sm">
                    {event.books.map((book, i) => (
                      <span key={book.id}>
                        {i > 0 && ', '}
                        <Link href={`/books/${book.slug}`} className="text-primary hover:underline">
                          {book.title}
                        </Link>
                      </span>
                    ))}
                  </p>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}

      {calendar.anniversaries.length > 0 && (
        <div className={hasEvents ? 'mt-6 border-t pt-4' : 'mt-4'}>
          <h3 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
            Юбилеи года
          </h3>
          <ul className="mt-2 space-y-1 text-sm">
            {calendar.anniversaries.map((a) => (
              <li key={a.book.id}>
                <Link href={`/books/${a.book.slug}`} className="hover:underline">
                  «{a.book.title}»
                </Link>
                {` — ${formatYears(a.years)}`}
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
