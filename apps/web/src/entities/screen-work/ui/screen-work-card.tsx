import type { ScreenWorkKind, ScreenWorkPoster as ScreenWorkPosterData } from '@bookhaven/api-client';
import type { ReactNode } from 'react';
import { screenWorkKindLabel } from '../lib/kind-label';
import { PosterAttribution } from './poster-attribution';
import { ScreenWorkPoster } from './screen-work-poster';

interface ScreenWorkCardProps {
  title: string;
  kind: ScreenWorkKind;
  year: number | null;
  poster: ScreenWorkPosterData | null;
  documentary?: boolean;
  director?: string | null;
  kinopoiskUrl?: string | null;
  imdbUrl?: string | null;
  /** Rendered under the metadata line (verdict and vote on the book page). */
  children?: ReactNode;
}

const linkClass = 'text-primary hover:underline';

/** Compact horizontal card: 72 px poster thumbnail, title, one metadata line, extra slot, poster credit. */
export function ScreenWorkCard({
  title,
  kind,
  year,
  poster,
  documentary,
  director,
  kinopoiskUrl,
  imdbUrl,
  children,
}: ScreenWorkCardProps) {
  const parts: ReactNode[] = [
    year ? <span key="year">{year}</span> : null,
    <span key="kind">{screenWorkKindLabel(kind)}</span>,
    documentary ? <span key="doc">Документальный</span> : null,
    director ? <span key="director">Режиссёр: {director}</span> : null,
    kinopoiskUrl ? (
      <a key="kp" href={kinopoiskUrl} target="_blank" rel="noopener noreferrer" className={linkClass}>
        Кинопоиск
      </a>
    ) : null,
    imdbUrl ? (
      <a key="imdb" href={imdbUrl} target="_blank" rel="noopener noreferrer" className={linkClass}>
        IMDb
      </a>
    ) : null,
  ].filter(Boolean);

  return (
    <div className="flex gap-3">
      <div className="w-[72px] shrink-0">
        <ScreenWorkPoster title={title} kind={kind} year={year} poster={poster} />
      </div>
      <div className="min-w-0 flex-1">
        <p className="line-clamp-2 font-medium leading-tight">{title}</p>
        <p className="mt-1 flex flex-wrap items-center gap-x-1.5 text-sm text-muted-foreground">
          {parts.flatMap((part, i) =>
            i === 0 ? [part] : [<span key={`sep-${i}`} aria-hidden="true">·</span>, part],
          )}
        </p>
        {children}
        <PosterAttribution poster={poster} className="mt-1 text-[11px] leading-tight" />
      </div>
    </div>
  );
}
