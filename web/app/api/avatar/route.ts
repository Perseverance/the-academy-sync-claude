import { NextRequest, NextResponse } from 'next/server'

// Simple in-memory cache for avatar images
const avatarCache = new Map<string, { data: ArrayBuffer; contentType: string; timestamp: number }>()
const CACHE_DURATION = 7 * 24 * 60 * 60 * 1000 // 7 days

export async function GET(request: NextRequest) {
  try {
    const url = request.nextUrl.searchParams.get('url')
    
    if (!url) {
      return new NextResponse('Missing URL parameter', { status: 400 })
    }
    
    // Check if the URL is from allowed domains
    const allowedDomains = [
      'googleusercontent.com',
      'lh3.google.com',
      'graph.facebook.com',
      'www.strava.com'
    ]
    
    const urlObj = new URL(url)
    const isAllowed = allowedDomains.some(domain => urlObj.hostname.includes(domain))
    
    if (!isAllowed) {
      return new NextResponse('Invalid avatar URL', { status: 400 })
    }
    
    // Check in-memory cache
    const cached = avatarCache.get(url)
    if (cached && Date.now() - cached.timestamp < CACHE_DURATION) {
      return new NextResponse(cached.data, {
        headers: {
          'Content-Type': cached.contentType,
          'Cache-Control': 'public, max-age=604800', // 7 days
        },
      })
    }
    
    // Fetch the image with a timeout
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), 5000) // 5 second timeout
    
    try {
      const response = await fetch(url, {
        signal: controller.signal,
        headers: {
          'User-Agent': 'TheAcademySync/1.0',
        },
      })
      
      clearTimeout(timeoutId)
      
      if (!response.ok) {
        // Return a transparent 1x1 pixel if the image fails to load
        const pixel = Buffer.from('R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7', 'base64')
        return new NextResponse(pixel, {
          headers: {
            'Content-Type': 'image/gif',
            'Cache-Control': 'public, max-age=3600', // 1 hour for errors
          },
        })
      }
      
      const contentType = response.headers.get('content-type') || 'image/jpeg'
      const data = await response.arrayBuffer()
      
      // Store in cache
      avatarCache.set(url, {
        data,
        contentType,
        timestamp: Date.now(),
      })
      
      // Clean up old cache entries
      if (avatarCache.size > 100) {
        const entries = Array.from(avatarCache.entries())
        entries.sort((a, b) => a[1].timestamp - b[1].timestamp)
        for (let i = 0; i < 20; i++) {
          avatarCache.delete(entries[i][0])
        }
      }
      
      return new NextResponse(data, {
        headers: {
          'Content-Type': contentType,
          'Cache-Control': 'public, max-age=604800', // 7 days
        },
      })
    } catch (error) {
      // Return transparent pixel on error
      const pixel = Buffer.from('R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7', 'base64')
      return new NextResponse(pixel, {
        headers: {
          'Content-Type': 'image/gif',
          'Cache-Control': 'public, max-age=3600', // 1 hour for errors
        },
      })
    }
  } catch (error) {
    return new NextResponse('Internal Server Error', { status: 500 })
  }
}