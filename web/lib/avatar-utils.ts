/**
 * Adds a daily cache parameter to avatar URLs to improve browser caching
 * while avoiding excessive cache busting that could trigger rate limits
 */
export function getCachedAvatarUrl(url: string | undefined | null): string {
  if (!url) return "/placeholder.svg"
  
  // Add daily cache parameter (changes once per day)
  const dailyCacheKey = Math.floor(Date.now() / (1000 * 60 * 60 * 24))
  
  // Check if URL already has query parameters
  const separator = url.includes('?') ? '&' : '?'
  return `${url}${separator}t=${dailyCacheKey}`
}