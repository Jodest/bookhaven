import type { PhotoAttribution as PhotoAttributionData } from '@bookhaven/api-client';

interface PhotoAttributionProps {
  attribution: PhotoAttributionData;
  className?: string;
}

const linkClass = 'underline-offset-2 hover:underline';

/**
 * Licence attribution of a Wikimedia Commons author photo:
 * «Фото: {author}, {license}, Wikimedia Commons». The licence links to its
 * URL when known; "Wikimedia Commons" links to the file page. The text is
 * stored verbatim (often Latin script) - see
 * docs/features/author-wikidata-enrichment.md.
 */
export function PhotoAttribution({ attribution, className }: PhotoAttributionProps) {
  const { author, license, licenseUrl, sourceUrl } = attribution;
  return (
    <p className={['text-xs text-muted-foreground', className].filter(Boolean).join(' ')}>
      Фото: {author ? `${author}, ` : ''}
      {licenseUrl ? (
        <a href={licenseUrl} target="_blank" rel="noopener noreferrer" className={linkClass}>
          {license}
        </a>
      ) : (
        license
      )}
      ,{' '}
      <a href={sourceUrl} target="_blank" rel="noopener noreferrer" className={linkClass}>
        Wikimedia Commons
      </a>
    </p>
  );
}
