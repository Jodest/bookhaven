/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  transpilePackages: ['@bookhaven/ui', '@bookhaven/api-client'],
  images: {
    // Cover copies are content-hashed (/covers/<hash>.<ext>) and the fallback
    // Open Library links are stable per id, so optimized images can be kept
    // for 30 days (the default is 60 s, per width, and lost on deploy).
    minimumCacheTTL: 2592000,
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
  experimental: {
    typedRoutes: true,
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
    ];
  },
};

module.exports = nextConfig;
