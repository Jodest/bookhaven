/**
 * Builds a catalog URL for `page` from the current query string, preserving
 * every other param (including repeated ones). Page 1 carries no `page` param.
 */
export function buildCatalogPageHref(
  pathname: string,
  searchParams: { toString(): string },
  page: number
): string {
  const params = new URLSearchParams(searchParams.toString());
  if (page <= 1) {
    params.delete('page');
  } else {
    params.set('page', String(page));
  }
  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
}
