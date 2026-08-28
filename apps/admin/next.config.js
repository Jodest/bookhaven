/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  transpilePackages: ['@bookhaven/ui', '@bookhaven/api-client', '@bookhaven/types'],
  experimental: {
    typedRoutes: true,
  },
};

module.exports = nextConfig;
