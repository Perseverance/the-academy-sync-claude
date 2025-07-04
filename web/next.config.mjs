/** @type {import('next').NextConfig} */
const nextConfig = {
  eslint: {
    ignoreDuringBuilds: true,
  },
  typescript: {
    ignoreBuildErrors: true,
  },
  images: {
    unoptimized: true,
  },
  // Use 'standalone' for Docker/server deployments, 'export' for static hosting
  output: process.env.BUILD_STANDALONE === 'true' ? 'standalone' : 'export',
  // Generate directory structure for clean URLs
  trailingSlash: true,
}

export default nextConfig