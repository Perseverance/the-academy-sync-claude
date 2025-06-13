"use client"

import { useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { Loader2 } from "lucide-react"
import { XCircle } from "lucide-react" // Import XCircle for error display
import { useToast } from "@/hooks/use-toast"
import { useAuth } from "@/components/auth-provider" // To ensure user context is available if needed

export const dynamic = 'force-dynamic'

export default function StravaCallbackPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { toast } = useToast()
  const { user, isLoading: authLoading } = useAuth() // Get user context

  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    // Wait for auth context to load and ensure user is available
    if (authLoading) {
      return
    }

    // If somehow user is not authenticated here, redirect to login
    // This page should only be accessed by authenticated users mid-flow
    if (!user) {
      router.replace("/") // Or your login page
      return
    }

    const code = searchParams.get("code")
    const scope = searchParams.get("scope") // Strava returns scope, useful for verification
    const errorParam = searchParams.get("error")

    const intendedRedirectPath = localStorage.getItem("stravaOAuthRedirect") || "/dashboard"
    localStorage.removeItem("stravaOAuthRedirect") // Clean up

    if (errorParam) {
      setError(`Strava authorization failed: ${errorParam}`)
      toast({
        title: "Strava Connection Error",
        description: `Could not connect to Strava. Reason: ${errorParam}`,
        variant: "destructive",
      })
      setIsLoading(false)
      // Optionally redirect after a delay
      setTimeout(() => router.replace(intendedRedirectPath), 3000)
      return
    }

    if (code) {
      // Send the code to your backend API
      const exchangeCode = async () => {
        try {
          // Replace with your actual backend API endpoint
          const response = await fetch("/api/v0-proxy/strava/exchange-token", {
            // Example proxy endpoint
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              // Include Authorization header if your backend API requires it for this user
              // e.g., Authorization: `Bearer ${userSessionToken}`
            },
            body: JSON.stringify({ code, scope }), // Send code and scope
          })

          if (!response.ok) {
            const errorData = await response.json().catch(() => ({ message: "Unknown error during token exchange." }))
            throw new Error(errorData.message || `Failed to exchange Strava code. Status: ${response.status}`)
          }

          // const result = await response.json(); // Process result if backend sends data

          toast({
            title: "Strava Connected!",
            description: "Your Strava account has been successfully linked.",
            variant: "default", // shadcn/ui success variant is 'default' or you can customize
          })
          // Redirect to dashboard or intended page
          // The dashboard should then re-fetch connection statuses to update UI
          router.replace(intendedRedirectPath)
        } catch (err: any) {
          console.error("Strava code exchange error:", err)
          setError(err.message || "An unexpected error occurred.")
          toast({
            title: "Strava Connection Failed",
            description: err.message || "Could not complete Strava connection. Please try again.",
            variant: "destructive",
          })
          setIsLoading(false)
          setTimeout(() => router.replace(intendedRedirectPath), 3000)
        }
      }
      exchangeCode()
    } else {
      setError("No authorization code received from Strava.")
      toast({
        title: "Strava Connection Error",
        description: "Did not receive necessary information from Strava. Please try again.",
        variant: "destructive",
      })
      setIsLoading(false)
      setTimeout(() => router.replace(intendedRedirectPath), 3000)
    }
  }, [searchParams, router, toast, user, authLoading])

  if (isLoading) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center bg-background p-4">
        <Loader2 className="h-12 w-12 animate-spin text-primary mb-4" />
        <p className="text-lg text-foreground">Connecting to Strava, please wait...</p>
        <p className="text-sm text-muted-foreground">You will be redirected shortly.</p>
      </div>
    )
  }

  if (error) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center bg-background p-4 text-center">
        <XCircle className="h-12 w-12 text-destructive mb-4" />
        <h1 className="text-2xl font-semibold text-destructive mb-2">Connection Failed</h1>
        <p className="text-lg text-foreground mb-4">{error}</p>
        <p className="text-sm text-muted-foreground">Redirecting you back...</p>
      </div>
    )
  }

  // Should ideally not reach here if loading/error states are handled properly
  // Or if redirect happens quickly.
  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-background p-4">
      <p className="text-lg text-foreground">Processing Strava Authorization...</p>
    </div>
  )
}
