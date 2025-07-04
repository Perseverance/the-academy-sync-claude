"use client"

import { useState, useEffect } from "react"
import Image from "next/image"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { getCachedAvatarUrl } from "@/lib/avatar-utils"
import { cn } from "@/lib/utils"

interface CachedAvatarProps {
  src?: string | null
  alt?: string
  fallback?: string
  className?: string
  size?: number
}

export function CachedAvatar({ 
  src, 
  alt = "Avatar", 
  fallback,
  className,
  size = 40 
}: CachedAvatarProps) {
  const [imageError, setImageError] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  
  // Reset error state when src changes
  useEffect(() => {
    setImageError(false)
    setIsLoading(true)
  }, [src])
  
  const cachedUrl = getCachedAvatarUrl(src)
  const initials = fallback || alt.split(' ').map(n => n[0]).join('').toUpperCase().slice(0, 2)
  
  return (
    <Avatar className={className}>
      {!imageError && src ? (
        <AvatarImage 
          src={cachedUrl} 
          alt={alt}
          onError={() => setImageError(true)}
          onLoad={() => setIsLoading(false)}
          style={{ opacity: isLoading ? 0 : 1 }}
          className="transition-opacity duration-200"
        />
      ) : null}
      <AvatarFallback className={cn("bg-primary/10 text-primary font-medium", className)}>
        {initials}
      </AvatarFallback>
    </Avatar>
  )
}

// Standard img tag version for places that need a regular img element
export function CachedAvatarImg({ 
  src, 
  alt = "Avatar", 
  className,
  onError,
  ...props 
}: React.ImgHTMLAttributes<HTMLImageElement> & { src?: string | null }) {
  const [imageError, setImageError] = useState(false)
  const cachedUrl = getCachedAvatarUrl(src)
  
  const handleError = (e: React.SyntheticEvent<HTMLImageElement, Event>) => {
    setImageError(true)
    onError?.(e)
  }
  
  if (imageError || !src) {
    return (
      <div 
        className={cn("bg-primary/10 rounded-full flex items-center justify-center", className)}
        style={{ width: props.width || 40, height: props.height || 40 }}
      >
        <span className="text-primary font-medium text-sm">
          {alt.split(' ').map(n => n[0]).join('').toUpperCase().slice(0, 2)}
        </span>
      </div>
    )
  }
  
  return (
    <img
      {...props}
      src={cachedUrl}
      alt={alt}
      className={className}
      onError={handleError}
      loading="lazy"
    />
  )
}