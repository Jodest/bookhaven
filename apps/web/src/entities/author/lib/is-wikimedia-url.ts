// Hosts that serve Wikimedia Commons thumbnails filled by Wikidata
// enrichment (docs/features/author-wikidata-enrichment.md). Such photos are
// rendered `unoptimized` so the browser hotlinks the thumbnail directly
// instead of the Next optimizer fetching Wikimedia without a
// policy-compliant User-Agent.
const WIKIMEDIA_HOSTS = new Set(['upload.wikimedia.org', 'thumb.wikimedia.org']);

export function isWikimediaUrl(url: string | null | undefined): boolean {
  if (!url) return false;
  try {
    const parsed = new URL(url);
    return parsed.protocol === 'https:' && WIKIMEDIA_HOSTS.has(parsed.hostname);
  } catch {
    return false;
  }
}
