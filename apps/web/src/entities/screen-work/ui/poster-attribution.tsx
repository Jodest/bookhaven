import type { ScreenWorkPoster } from '@bookhaven/api-client';

interface PosterAttributionProps {
  poster: ScreenWorkPoster | null;
  className?: string;
}

const linkClass = 'underline-offset-2 hover:underline';

/**
 * Credit of a Wikimedia Commons poster: «Постер: {author}, {license},
 * Wikimedia Commons». The licence links to its URL when known and
 * "Wikimedia Commons" to the file page. Uploaded and TMDB posters have no
 * caption, so nothing is rendered for them.
 */
export function PosterAttribution({ poster, className }: PosterAttributionProps) {
  if (!poster || poster.source !== 'commons') return null;
  const { author, license, license_url, source_url } = poster;
  return (
    <p className={['text-xs text-muted-foreground', className].filter(Boolean).join(' ')}>
      Постер: {author ? `${author}, ` : ''}
      {license ? (
        <>
          {license_url ? (
            <a href={license_url} target="_blank" rel="noopener noreferrer" className={linkClass}>
              {license}
            </a>
          ) : (
            license
          )}
          ,{' '}
        </>
      ) : null}
      {source_url ? (
        <a href={source_url} target="_blank" rel="noopener noreferrer" className={linkClass}>
          Wikimedia Commons
        </a>
      ) : (
        'Wikimedia Commons'
      )}
    </p>
  );
}
