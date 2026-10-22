/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  transpilePackages: ['@bookhaven/ui', '@bookhaven/api-client'],
  experimental: {
    typedRoutes: true,
  },
  // Old Import/Catalog URLs → their exact method sub-page, and bare Import
  // entity paths → the tab's default sub-page (must equal the tab href in
  // src/shared/config/navigation.ts; enforced by entity-redirects.test.tsx).
  // A server redirect() in the page streams a 200 under the client root
  // layout, so the 307 lives here. docs/features/import-by-entity.md and
  // import-method-subpages.md. Not permanent: later admin unification phases
  // may move these pages again.
  async redirects() {
    return [
      { source: '/import/books', destination: '/import/books/wikidata', permanent: false },
      { source: '/import/authors', destination: '/import/authors/manual', permanent: false },
      { source: '/import/genres', destination: '/import/genres/manual', permanent: false },
      { source: '/import/wikidata', destination: '/import/books/wikidata', permanent: false },
      { source: '/import/file', destination: '/import/books/file', permanent: false },
      { source: '/import/author', destination: '/import/authors/wikidata', permanent: false },
      { source: '/catalog/books/new', destination: '/import/books/manual', permanent: false },
    ];
  },
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: `${process.env.API_URL || process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/:path*`,
      },
      // Keeps /covers/... links working on the admin origin too.
      {
        source: '/covers/:path*',
        destination: `${process.env.API_URL || process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/covers/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
