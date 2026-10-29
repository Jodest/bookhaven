// Book covers on these hosts are only linked, never copied by the backend
// (its COVERS_LINK_ONLY_HOSTS list, default fantlab.ru and *.fantlab.ru). They
// are rendered `unoptimized` so the visitor's browser loads them straight from
// the source and the Next image optimizer never fetches or caches them. Keep
// this in step with COVERS_LINK_ONLY_HOSTS, and empty it when a host moves to
// the download allowlist. The hosts are deliberately not in
// `images.remotePatterns` (next.config.js).
const HOTLINK_COVER_DOMAINS = ['fantlab.ru'];

export function isHotlinkCoverUrl(url: string | null | undefined): boolean {
  if (!url) return false;
  try {
    const parsed = new URL(url);
    if (parsed.protocol !== 'https:') return false;
    const host = parsed.hostname.toLowerCase();
    return HOTLINK_COVER_DOMAINS.some((d) => host === d || host.endsWith(`.${d}`));
  } catch {
    return false;
  }
}
