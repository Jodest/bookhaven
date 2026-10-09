import { createRequire } from 'node:module';
import path from 'node:path';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getTabHref } from '@/shared/config';
import BooksPage from './books/page';
import AuthorsPage from './authors/page';
import GenresPage from './genres/page';

const redirectMock = vi.fn();

vi.mock('next/navigation', () => ({
  redirect: (url: string) => redirectMock(url),
}));

interface Redirect {
  source: string;
  destination: string;
  permanent: boolean;
}

async function configRedirects(): Promise<Redirect[]> {
  const require = createRequire(import.meta.url);
  const nextConfig = require(path.resolve(__dirname, '../../../next.config.js')) as {
    redirects: () => Promise<Redirect[]>;
  };
  return nextConfig.redirects();
}

// Bare entity paths go to the tab's default sub-page (the tab href in
// shared/config/navigation.ts), docs/features/import-method-subpages.md. The
// HTTP 307 comes from next.config.js redirects(); the pages' server redirect()
// is a fallback (it streams a 200 under the client root layout).
describe('bare /import/{entity} paths', () => {
  beforeEach(() => {
    redirectMock.mockReset();
  });

  it.each([
    ['books', BooksPage, '/import/books/wikidata'],
    ['authors', AuthorsPage, '/import/authors/manual'],
    ['genres', GenresPage, '/import/genres/manual'],
  ])('/import/%s page redirects to %s', (_entity, Page, target) => {
    Page();
    expect(redirectMock).toHaveBeenCalledTimes(1);
    expect(redirectMock).toHaveBeenCalledWith(target);
  });

  it.each(['books', 'authors', 'genres'])(
    'next.config.js redirects /import/%s to the tab href (temporary)',
    async (entity) => {
      const rule = (await configRedirects()).find((r) => r.source === `/import/${entity}`);
      expect(rule).toEqual({
        source: `/import/${entity}`,
        destination: getTabHref(`import.${entity}`),
        permanent: false,
      });
    }
  );

  it('legacy URLs redirect to their exact method sub-page', async () => {
    const bySource = Object.fromEntries((await configRedirects()).map((r) => [r.source, r.destination]));
    expect(bySource).toMatchObject({
      '/import/wikidata': '/import/books/wikidata',
      '/import/file': '/import/books/file',
      '/import/author': '/import/authors/wikidata',
      '/catalog/books/new': '/import/books/manual',
    });
  });
});
