/** Page size of /top/{slug}; the API default and the value the page numbers are based on. */
export const COLLECTION_PAGE_SIZE = 50;

/** Link to the /top/{slug} page that shows the given 1-based place. */
export function collectionPlaceHref(slug: string, place: number): string {
  const page = Math.ceil(place / COLLECTION_PAGE_SIZE);
  return page <= 1 ? `/top/${slug}` : `/top/${slug}?page=${page}`;
}
