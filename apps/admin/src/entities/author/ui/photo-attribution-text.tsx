import type { AuthorPhotoAttribution } from '@bookhaven/api-client';

interface PhotoAttributionTextProps {
  attribution: AuthorPhotoAttribution;
  className?: string;
}

/**
 * Read-only Commons attribution of a Wikidata-enriched photo:
 * «Фото: {author}, {license}, Wikimedia Commons» plus a link to the file
 * page. Attribution is not editable; it is cleared when the photo URL is
 * changed (docs/features/author-wikidata-enrichment.md rule 7).
 */
export function PhotoAttributionText({ attribution, className }: PhotoAttributionTextProps) {
  const { author, license, license_url: licenseUrl, source_url: sourceUrl } = attribution;
  return (
    <p className={['text-xs text-muted-foreground', className].filter(Boolean).join(' ')}>
      Фото: {author ? `${author}, ` : ''}
      {licenseUrl ? (
        <a href={licenseUrl} target="_blank" rel="noopener noreferrer" className="underline">
          {license}
        </a>
      ) : (
        license
      )}
      , Wikimedia Commons ·{' '}
      <a href={sourceUrl} target="_blank" rel="noopener noreferrer" className="underline">
        Страница файла
      </a>
    </p>
  );
}
