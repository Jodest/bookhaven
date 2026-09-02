/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  transpilePackages: ['@bookhaven/ui', '@bookhaven/api-client', '@bookhaven/types'],
  images: {
    remotePatterns: [
      {
        protocol: 'https',
        hostname: 'covers.openlibrary.org',
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
    ];
  },
};

module.exports = nextConfig;
