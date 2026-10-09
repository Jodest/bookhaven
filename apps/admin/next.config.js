/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  transpilePackages: ['@bookhaven/ui', '@bookhaven/api-client'],
  experimental: {
    typedRoutes: true,
  },
  // Old Import/Catalog URLs, docs/features/import-by-entity.md. Not permanent:
  // later admin unification phases may move these pages again.
  async redirects() {
    return [
      { source: '/import/wikidata', destination: '/import/books', permanent: false },
      { source: '/import/file', destination: '/import/books', permanent: false },
      { source: '/import/author', destination: '/import/authors', permanent: false },
      { source: '/catalog/books/new', destination: '/import/books', permanent: false },
    ];
  },
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
