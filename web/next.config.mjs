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
  // Required for GCS static hosting to properly serve routes
  trailingSlash: true,
}

export default nextConfig