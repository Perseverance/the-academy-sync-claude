/**
 * Manages avatar URLs with intelligent caching to prevent rate limiting
 */
export function getCachedAvatarUrl(url: string | undefined | null, useProxy = false): string {
  if (!url) return "/placeholder.svg"
  
  // Option to use our proxy API for maximum rate limit protection
  if (useProxy && (url.includes('googleusercontent.com') || url.includes('google.com'))) {
    return `/api/avatar?url=${encodeURIComponent(url)}`
  }
  
  // For Google profile pictures, use a weekly cache to reduce API calls
  if (url.includes('googleusercontent.com') || url.includes('google.com')) {
    // Use weekly cache key for Google avatars (changes once per week)
    const weeklyCacheKey = Math.floor(Date.now() / (1000 * 60 * 60 * 24 * 7))
    
    // Extract the size parameter if it exists
    const sizeMatch = url.match(/[?&]sz?=(\d+)/)
    const size = sizeMatch ? sizeMatch[1] : '200'
    
    // For Google URLs, we'll use a more stable caching approach
    // Remove any existing query parameters and add our cache key
    const baseUrl = url.split('?')[0]
    return `${baseUrl}?sz=${size}&t=${weeklyCacheKey}`
  }
  
  // For other URLs, use daily cache parameter
  const dailyCacheKey = Math.floor(Date.now() / (1000 * 60 * 60 * 24))
  
  // Check if URL contains a hash fragment
  const hashIndex = url.indexOf('#')
  let baseUrl = url
  let hashFragment = ''
  
  if (hashIndex !== -1) {
    baseUrl = url.substring(0, hashIndex)
    hashFragment = url.substring(hashIndex)
  }
  
  // Check if base URL already has query parameters
  const separator = baseUrl.includes('?') ? '&' : '?'
  return `${baseUrl}${separator}t=${dailyCacheKey}${hashFragment}`
}

/**
 * Preloads an avatar image to improve perceived performance
 */
export function preloadAvatar(url: string | undefined | null): void {
  if (!url) return
  
  const cachedUrl = getCachedAvatarUrl(url)
  const link = document.createElement('link')
  link.rel = 'preload'
  link.as = 'image'
  link.href = cachedUrl
  document.head.appendChild(link)
}