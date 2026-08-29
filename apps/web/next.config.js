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
};

module.exports = nextConfig;
