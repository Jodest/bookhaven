'use client';

import type { ScreenWorkKind, ScreenWorkPoster as ScreenWorkPosterData } from '@bookhaven/api-client';
import Image from 'next/image';
import { useState } from 'react';
import { screenWorkKindIcon } from '../lib/kind-icon';

interface ScreenWorkPosterProps {
  title: string;
  kind: ScreenWorkKind;
  year: number | null;
  poster: ScreenWorkPosterData | null;
}

/**
 * 2:3 poster frame of a film or series. Without a poster, or when the image
 * fails to load, a placeholder with the type icon and the year is shown.
 * Site paths (/posters/...) go through next/image; absolute Commons and TMDB
 * URLs are rendered unoptimized (like author photos).
 */
export function ScreenWorkPoster({ title, kind, year, poster: posterProp }: ScreenWorkPosterProps) {
  const poster = posterProp ?? null;
  const [failedUrl, setFailedUrl] = useState<string | null>(null);
  const showImage = poster !== null && failedUrl !== poster.url;
  const Icon = screenWorkKindIcon(kind);

  return (
    <div className="relative aspect-[2/3] w-full overflow-hidden rounded-md border bg-muted">
      {showImage ? (
        <Image
          src={poster.url}
          alt={title}
          fill
          sizes="(min-width: 1024px) 200px, (min-width: 640px) 30vw, 45vw"
          unoptimized={/^https?:\/\//.test(poster.url)}
          className="object-cover"
          onError={() => setFailedUrl(poster.url)}
        />
      ) : (
        <div
          data-testid="poster-placeholder"
          className="flex h-full w-full flex-col items-center justify-center gap-2 bg-gradient-to-br from-muted to-secondary/60 p-2 text-muted-foreground"
        >
          <Icon className="h-10 w-10" aria-hidden="true" />
          {year ? <span className="text-sm font-medium">{year}</span> : null}
        </div>
      )}
    </div>
  );
}
