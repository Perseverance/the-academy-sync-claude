"use client"

import type React from "react"
import { createContext, useContext, useState, useEffect, useRef } from "react"
import { useRouter, usePathname } from "next/navigation"
import type { LogEntry } from "@/components/activity-log" // Assuming LogEntry type is in ActivityLog
import { authService, type User } from "@/services/auth"
import { stravaService } from "@/services/strava"
import { configService } from "@/services/config"
import { syncService, SyncError } from "@/services/SyncService"
import { settingsService } from "@/services/settings"
import { useToast } from "@/hooks/use-toast"

export type ServiceStatus = "Connected" | "NotConnected" | "ReauthorizationNeeded"
export type SpreadsheetConfigStatus = "Configured" | "NotConfigured" | "Disabled"
export type SyncStatus = "Ready" | "Processing" | "Disabled"

interface AppState {
  user: User | null
  isAuthLoading: boolean
  googleStatus: ServiceStatus
  stravaStatus: ServiceStatus
  stravaUserName?: string
  stravaAvatarUrl?: string
  spreadsheetStatus: SpreadsheetConfigStatus
  spreadsheetUrl?: string
  manualSyncStatus: SyncStatus
  activityLogs: LogEntry[]
  isLogsLoading: boolean
}

interface AppActions {
  signIn: () => Promise<void>
  signOut: () => void
  connectStrava: () => Promise<void>
  disconnectStrava: () => Promise<void>
  reauthorizeStrava: () => Promise<void>
  saveSpreadsheet: (url: string) => Promise<void>
  changeSpreadsheet: () => void
  triggerManualSync: () => Promise<void>
  setGoogleStatus: (status: ServiceStatus) => void // For external updates if needed
  updateUserSettings: (settings: { automation_enabled: boolean; email_notifications_enabled: boolean }) => Promise<void>
}

const AppStateContext = createContext<
  | {
      state: AppState
      actions: AppActions
    }
  | undefined
>(undefined)


export function AppStateProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const pathname = usePathname()
  const isMountedRef = useRef(true)
  const pollingTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const { toast } = useToast()

  const [state, setState] = useState<AppState>({
    user: null,
    isAuthLoading: true,
    googleStatus: "Connected", // Assume Google is connected on sign-in
    stravaStatus: "NotConnected",
    spreadsheetStatus: "Disabled",
    manualSyncStatus: "Disabled",
    activityLogs: [],
    isLogsLoading: true,
  })


  // Authentication - Check session with backend
  useEffect(() => {
    const checkAuthStatus = async () => {
      try {
        const { isAuthenticated, user } = await authService.checkAuthStatus()
        setState((s) => ({ 
          ...s, 
          user: user, 
          isAuthLoading: false,
          googleStatus: isAuthenticated ? "Connected" : "NotConnected",
          // Initialize Strava status based on user data
          stravaStatus: user?.has_strava_connection ? "Connected" : "NotConnected",
          stravaUserName: user?.strava_athlete_name,
          stravaAvatarUrl: user?.strava_profile_picture_url,
          // Initialize spreadsheet status based on user data
          spreadsheetStatus: user?.has_sheets_connection ? "Configured" : 
                             user?.has_strava_connection ? "NotConfigured" : "Disabled",
          // Convert spreadsheet_id to Google Sheets URL
          spreadsheetUrl: user?.spreadsheet_id ? 
                         `https://docs.google.com/spreadsheets/d/${user.spreadsheet_id}` : undefined,
          // Use activity logs from user data
          activityLogs: user?.recent_activity_logs || [],
          isLogsLoading: false
        }))

        // Update timezone if user is authenticated (fire-and-forget)
        if (user) {
          const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone
          configService.updateUserTimezone(browserTimezone)
        }
      } catch (error) {
        console.error('Error checking auth status:', error)
        setState((s) => ({ ...s, isAuthLoading: false }))
      }
    }

    checkAuthStatus()
  }, [])

  useEffect(() => {
    if (!state.isAuthLoading) {
      if (state.user && pathname === "/") {
        router.push("/dashboard")
      } else if (!state.user && pathname !== "/") {
        router.push("/")
      }
    }
  }, [state.user, state.isAuthLoading, pathname, router])

  // Derived states based on user flow
  useEffect(() => {
    if (state.user) {
      // If Strava connects, enable Spreadsheet config
      if (state.stravaStatus === "Connected" && state.spreadsheetStatus === "Disabled") {
        setState((s) => ({ ...s, spreadsheetStatus: "NotConfigured" }))
      } else if (state.stravaStatus !== "Connected" && state.spreadsheetStatus !== "Disabled") {
        setState((s) => ({ ...s, spreadsheetStatus: "Disabled", spreadsheetUrl: undefined }))
      }

      // If Spreadsheet is configured, enable Manual Sync
      if (state.spreadsheetStatus === "Configured" && state.manualSyncStatus === "Disabled") {
        setState((s) => ({ ...s, manualSyncStatus: "Ready" }))
      } else if (state.spreadsheetStatus !== "Configured" && state.manualSyncStatus !== "Disabled") {
        setState((s) => ({ ...s, manualSyncStatus: "Disabled" }))
      }
    } else {
      // Reset if user logs out
      setState((s) => ({
        ...s,
        stravaStatus: "NotConnected",
        stravaUserName: undefined,
        stravaAvatarUrl: undefined,
        spreadsheetStatus: "Disabled",
        spreadsheetUrl: undefined,
        manualSyncStatus: "Disabled",
        activityLogs: [],
      }))
    }
  }, [state.user, state.stravaStatus, state.spreadsheetStatus])

  const signIn = async () => {
    setState((s) => ({ ...s, isAuthLoading: true }))
    try {
      // Initiate Google OAuth flow - this will redirect to Google
      await authService.initiateGoogleOAuth()
      // Note: After successful OAuth, user will be redirected back to our app
      // The auth status will be checked again by the useEffect above
    } catch (error) {
      console.error('Error during sign in:', error)
      setState((s) => ({ ...s, isAuthLoading: false }))
    }
  }

  const signOut = async () => {
    try {
      await authService.signOut()
    } catch (error) {
      console.error('Error during sign out:', error)
    }
    
    // Clear local state regardless of API call success
    setState((s) => ({ 
      ...s, 
      user: null,
      googleStatus: "NotConnected",
      stravaStatus: "NotConnected",
      stravaUserName: undefined,
      stravaAvatarUrl: undefined,
      spreadsheetStatus: "Disabled",
      spreadsheetUrl: undefined,
      manualSyncStatus: "Disabled",
      activityLogs: []
    }))
    router.push("/")
  }

  const connectStrava = async () => {
    try {
      // Initiate Strava OAuth flow - this will redirect to Strava
      await stravaService.initiateStravaConnection()
      // Note: After successful OAuth, user will be redirected back to our app
      // The Strava connection status will be updated when the user returns from callback
    } catch (error) {
      console.error('Error during Strava connection:', error)
      // Could show a toast notification here in the future
    }
  }

  const disconnectStrava = async () => {
    try {
      await stravaService.disconnectStrava()
      
      // Update local state to reflect disconnection
      setState((s) => ({
        ...s,
        stravaStatus: "NotConnected",
        stravaUserName: undefined,
        stravaAvatarUrl: undefined,
        // Spreadsheet and sync become disabled as per flow
        spreadsheetStatus: "Disabled",
        spreadsheetUrl: undefined,
        manualSyncStatus: "Disabled",
      }))
    } catch (error) {
      console.error('Error during Strava disconnection:', error)
      // Could show a toast notification here in the future
    }
  }

  const reauthorizeStrava = async () => {
    // For now, reauthorization uses the same flow as initial connection
    await connectStrava()
  }

  const saveSpreadsheet = async (url: string) => {
    try {
      // Call the real API to save the spreadsheet configuration
      await configService.setSpreadsheetUrl(url)
      
      // Check if component is still mounted before updating state
      if (!isMountedRef.current) {
        return
      }
      
      // For now, use the provided URL as canonical until API returns canonical URL
      // TODO: Update when backend returns canonical spreadsheet URL in response
      setState((s) => ({ 
        ...s, 
        spreadsheetStatus: "Configured", 
        spreadsheetUrl: url 
      }))
    } catch (error) {
      console.error('Failed to save spreadsheet configuration:', error)
      // Let the component handle the error display
      throw error
    }
  }

  const changeSpreadsheet = () => {
    setState((s) => ({ ...s, spreadsheetStatus: "NotConfigured" }))
  }

  const triggerManualSync = async () => {
    // Only update state if component is still mounted
    if (isMountedRef.current) {
      setState((s) => ({ ...s, manualSyncStatus: "Processing" }))
    }
    
    try {
      // Store the current activity logs count before sync
      const initialActivityCount = state.activityLogs.length
      
      // Call the actual sync API
      const response = await syncService.triggerManualSync()
      
      // Manual sync triggered successfully
      
      // Show success toast notification
      toast({
        title: "Sync Started",
        description: "Manual sync has been triggered successfully and is now processing.",
      })
      
      // Start polling for new activity logs
      let pollCount = 0
      const maxPolls = 6
      const pollInterval = 5000 // 5 seconds
      
      const pollForNewActivity = async () => {
        pollCount++
        
        try {
          // Fetch updated user data
          const user = await authService.getCurrentUser()
          
          if (user && user.recent_activity_logs) {
            const newActivityCount = user.recent_activity_logs.length
            
            // Check if we found new activity logs
            if (newActivityCount > initialActivityCount) {
              
              // Calculate total activities from the new logs
              let totalActivitiesProcessed = 0
              // Get only the new logs that were added since sync started
              // The array is in reverse chronological order (newest first)
              // So we need to get the first N logs where N = newActivityCount - initialActivityCount
              const numNewLogs = newActivityCount - initialActivityCount
              const newLogs = user.recent_activity_logs.slice(0, numNewLogs)
              
              
              for (const log of newLogs) {
                // Parse processed count from summary text
                // Examples: "1 activity found, 1 processed", "6 activities found, 1 processed"
                const processedMatch = log.summary.match(/(\d+) processed/)
                if (processedMatch) {
                  const processed = parseInt(processedMatch[1], 10)
                  totalActivitiesProcessed += processed
                } else {
                  // Fallback: if no "processed" count, check if activities were found
                  const activityMatch = log.summary.match(/(\d+) activit(?:y|ies) found/)
                  if (activityMatch) {
                    const found = parseInt(activityMatch[1], 10)
                    totalActivitiesProcessed += found
                  }
                }
              }
              
              // Update state with new data
              if (isMountedRef.current) {
                setState((s) => ({
                  ...s,
                  activityLogs: user.recent_activity_logs,
                  manualSyncStatus: "Ready"
                }))
              }
              
              // Show success notification with accurate activity count
              const logText = newLogs.length === 1 ? 'log' : 'logs'
              const activityText = totalActivitiesProcessed === 1 ? 'activity' : 'activities'
              
              toast({
                title: "Sync Completed",
                description: totalActivitiesProcessed > 0 
                  ? `Successfully synced ${totalActivitiesProcessed} ${activityText} in ${newLogs.length} ${logText}.`
                  : `Sync completed with ${newLogs.length} ${logText}, but no new activities were found.`,
              })
              
              return // Stop polling
            }
          }
          
          // Continue polling if we haven't reached the limit
          if (pollCount < maxPolls && isMountedRef.current) {
            pollingTimeoutRef.current = setTimeout(pollForNewActivity, pollInterval)
          } else {
            // Max polls reached or component unmounted
            // Stopped polling after max attempts
            if (isMountedRef.current) {
              setState((s) => ({ ...s, manualSyncStatus: "Ready" }))
            }
          }
        } catch (error) {
          console.error('Error polling for activity updates:', error)
          // Don't show error toast for polling failures, just stop polling
          if (isMountedRef.current) {
            setState((s) => ({ ...s, manualSyncStatus: "Ready" }))
          }
        }
      }
      
      // Start polling after a short delay to give the backend time to process
      pollingTimeoutRef.current = setTimeout(pollForNewActivity, pollInterval)
      
    } catch (error) {
      console.error('Manual sync failed to start:', error)
      
      let errorMessage = 'Failed to trigger sync. Please try again.'
      if (error instanceof SyncError) {
        errorMessage = error.message
      } else if (error instanceof Error) {
        errorMessage = error.message
      }
      
      // Show error toast
      // TODO: Create a general error toast utility function to standardize error display
      toast({
        title: "Sync Failed",
        description: errorMessage,
        variant: "destructive",
      })
      
      // Only update state if component is still mounted
      if (isMountedRef.current) {
        setState((s) => ({
          ...s,
          manualSyncStatus: "Ready",
        }))
      }
    }
  }

  const setGoogleStatus = (status: ServiceStatus) => {
    setState((s) => ({ ...s, googleStatus: status }))
  }

  const updateUserSettings = async (settings: { automation_enabled: boolean; email_notifications_enabled: boolean }) => {
    try {
      const updatedUser = await settingsService.updateSettings(settings)
      setState((s) => ({ ...s, user: updatedUser }))
      
      // Update sync status based on automation setting
      updateServiceStatuses(updatedUser)
    } catch (error: any) {
      console.error('Failed to update settings:', error)
      throw error
    }
  }


  // Remove the loading state after initial load
  useEffect(() => {
    if (state.user && state.isLogsLoading) {
      // Just remove the loading state - we already have the real logs from the auth check
      setState((s) => ({ ...s, isLogsLoading: false }))
    }
  }, [state.user, state.isLogsLoading])

  const actions: AppActions = {
    signIn,
    signOut,
    connectStrava,
    disconnectStrava,
    reauthorizeStrava,
    saveSpreadsheet,
    changeSpreadsheet,
    triggerManualSync,
    setGoogleStatus,
    updateUserSettings,
  }

  // Update timezone when window regains focus (in case user changed system timezone)
  useEffect(() => {
    const handleFocus = () => {
      if (state.user) {
        const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone
        configService.updateUserTimezone(browserTimezone)
      }
    }

    window.addEventListener('focus', handleFocus)
    return () => {
      window.removeEventListener('focus', handleFocus)
    }
  }, [state.user])

  // Cleanup effect to mark component as unmounted and clear timeouts
  useEffect(() => {
    return () => {
      isMountedRef.current = false
      // Clear any pending polling timeout
      if (pollingTimeoutRef.current) {
        clearTimeout(pollingTimeoutRef.current)
        pollingTimeoutRef.current = null
      }
    }
  }, [])

  return <AppStateContext.Provider value={{ state, actions }}>{children}</AppStateContext.Provider>
}

export function useAppState() {
  const context = useContext(AppStateContext)
  if (context === undefined) {
    throw new Error("useAppState must be used within an AppStateProvider")
  }
  return context
}
