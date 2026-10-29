const { withSentryConfig } = require('@sentry/nextjs/config');
const { securityHeaders } = require('./security-headers');

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  transpilePackages: ['@bookhaven/ui', '@bookhaven/api-client'],
  images: {
    // Cover copies are content-hashed (/covers/<hash>.<ext>) and the fallback
    // Open Library links are stable per id, so optimized images can be kept
    // for 30 days (the default is 60 s, per width, and lost on deploy).
    minimumCacheTTL: 2592000,
    // fantlab.ru is deliberately NOT listed: its covers are link-only (backend
    // COVERS_LINK_ONLY_HOSTS) and rendered `unoptimized` (shared/lib/hotlink-cover),
    // because the optimizer would fetch and cache them for 30 days, which is a
    // stored copy we have no permission for.
    remotePatterns: [
      {
        protocol: 'https',
        hostname: 'covers.openlibrary.org',
      },
      // Wikimedia Commons author photos (Wikidata enrichment). Rendered
      // `unoptimized` (see entities/author isWikimediaUrl); Commons returns
      // thumbnails on both hosts.
      {
        protocol: 'https',
        hostname: 'upload.wikimedia.org',
        pathname: '/wikipedia/commons/**',
      },
      {
        protocol: 'https',
        hostname: 'thumb.wikimedia.org',
        pathname: '/wikipedia/commons/**',
      },
    ],
  },
  // Stable in Next 15.5 (was experimental).
  typedRoutes: true,
  // Excludes the proxied /api, /covers and /posters so backend headers are not duplicated.
  async headers() {
    return [
      {
        source: '/((?!api/|covers/|posters/).*)',
        headers: securityHeaders({
          isDev: process.env.NODE_ENV === 'development',
          apiUrl: process.env.NEXT_PUBLIC_API_URL,
          noindex: false,
        }),
      },
    ];
  },
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: `${process.env.API_URL || 'http://localhost:8080'}/api/:path*`,
      },
      // Self-hosted book covers are served by the backend (public files).
      {
        source: '/covers/:path*',
        destination: `${process.env.API_URL || 'http://localhost:8080'}/covers/:path*`,
      },
      // Uploaded screen work posters are served by the backend (public files).
      {
        source: '/posters/:path*',
        destination: `${process.env.API_URL || 'http://localhost:8080'}/posters/:path*`,
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
