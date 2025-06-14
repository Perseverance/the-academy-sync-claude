"use client"

import type React from "react"
import { useAppState } from "@/context/app-state-provider"
import { useRouter } from "next/navigation"
import { useEffect } from "react"

interface ProtectedRouteProps {
  children: React.ReactNode
  fallback?: React.ReactNode
}

/**
 * ProtectedRoute component ensures that only authenticated users can access wrapped content.
 * 
 * This component checks the current authentication state from the AppStateProvider.
 * If the user is not authenticated, it redirects to the sign-in page.
 * If authentication is still loading, it shows the fallback component or nothing.
 * 
 * @param children - The content to render if user is authenticated
 * @param fallback - Optional content to show while authentication is loading
 */
export function ProtectedRoute({ children, fallback }: ProtectedRouteProps) {
  const { state } = useAppState()
  const router = useRouter()

  useEffect(() => {
    // Only redirect if we're done loading and user is not authenticated
    if (!state.isAuthLoading && !state.user) {
      router.push("/")
    }
  }, [state.isAuthLoading, state.user, router])

  // Show fallback while authentication is loading
  if (state.isAuthLoading) {
    return <>{fallback || null}</>
  }

  // Show nothing if user is not authenticated (will redirect)
  if (!state.user) {
    return null
  }

  // User is authenticated, render children
  return <>{children}</>
}