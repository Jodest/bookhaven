const { withSentryConfig } = require('@sentry/nextjs/config');
const { securityHeaders } = require('./security-headers');

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  transpilePackages: ['@bookhaven/ui', '@bookhaven/api-client'],
  // Stable in Next 15.5 (was experimental).
  typedRoutes: true,
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
  // Excludes the proxied /api, /covers and /posters so backend headers are not duplicated.
  async headers() {
    return [
      {
        source: '/((?!api/|covers/|posters/).*)',
        headers: securityHeaders({
          isDev: process.env.NODE_ENV === 'development',
          apiUrl: process.env.NEXT_PUBLIC_API_URL,
          noindex: true,
        }),
      },
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
      // Uploaded screen work posters (the work edit page previews them).
      {
        source: '/posters/:path*',
        destination: `${process.env.API_URL || process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/posters/:path*`,
      },
    ];
  },
};

// Server-side error tracking only (docs/features/error-tracking.md): no
// tunnelRoute, no source map upload, no client SDK file. The DSN is read at
// runtime (SENTRY_DSN); without it the SDK is never initialised.
module.exports = withSentryConfig(nextConfig, {
  silent: true,
  telemetry: false,
  sourcemaps: { disable: true },
  autoInstrumentMiddleware: false,
  disableLogger: true,
  widenClientFileUpload: false,
});
