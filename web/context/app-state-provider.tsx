"use client"

import type React from "react"
import { createContext, useContext, useState, useEffect } from "react"
import { useRouter, usePathname } from "next/navigation"
import type { LogEntry } from "@/components/activity-log" // Assuming LogEntry type is in ActivityLog
import { authService, type User } from "@/services/auth"

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
}

const AppStateContext = createContext<
  | {
      state: AppState
      actions: AppActions
    }
  | undefined
>(undefined)

const mockLogs: LogEntry[] = [
  { id: "1", date: new Date(Date.now() - 86400000).toISOString(), status: "Success", summary: "Synced 5 activities." },
  {
    id: "2",
    date: new Date(Date.now() - 2 * 86400000).toISOString(),
    status: "Failure",
    summary: "Strava API timeout.",
  },
  {
    id: "3",
    date: new Date(Date.now() - 3 * 86400000).toISOString(),
    status: "SuccessWithWarning",
    summary: "Synced 3 activities, 1 skipped (duplicate).",
  },
]

export function AppStateProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const pathname = usePathname()

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
          // Initialize spreadsheet status based on user data
          spreadsheetStatus: user?.has_sheets_connection ? "Configured" : 
                             user?.has_strava_connection ? "NotConfigured" : "Disabled",
          // Use activity logs from user data, fallback to mock data if empty
          activityLogs: user?.recent_activity_logs?.length ? user.recent_activity_logs : mockLogs,
          isLogsLoading: false
        }))
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
    // Simulate OAuth flow and success
    await new Promise((resolve) => setTimeout(resolve, 1500))
    setState((s) => ({
      ...s,
      stravaStatus: "Connected",
      stravaUserName: "Strava Runner",
      stravaAvatarUrl: "/placeholder.svg?width=40&height=40",
    }))
  }

  const disconnectStrava = async () => {
    await new Promise((resolve) => setTimeout(resolve, 1000))
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
  }

  const reauthorizeStrava = async () => {
    await new Promise((resolve) => setTimeout(resolve, 1500))
    setState((s) => ({ ...s, stravaStatus: "Connected" })) // Assume reauth is successful
  }

  const saveSpreadsheet = async (url: string) => {
    await new Promise((resolve) => setTimeout(resolve, 1000))
    setState((s) => ({ ...s, spreadsheetStatus: "Configured", spreadsheetUrl: url }))
  }

  const changeSpreadsheet = () => {
    setState((s) => ({ ...s, spreadsheetStatus: "NotConfigured" }))
  }

  const triggerManualSync = async () => {
    setState((s) => ({ ...s, manualSyncStatus: "Processing" }))
    await new Promise((resolve) => setTimeout(resolve, 3000)) // Simulate sync
    // Add a new log entry
    const newLog: LogEntry = {
      id: String(Date.now()),
      date: new Date().toISOString(),
      status: Math.random() > 0.3 ? "Success" : "Failure",
      summary:
        Math.random() > 0.3
          ? "Manual sync completed: 2 new activities."
          : "Manual sync failed: Could not reach Google Sheets.",
    }
    setState((s) => ({
      ...s,
      manualSyncStatus: "Ready",
      activityLogs: [newLog, ...s.activityLogs.slice(0, 19)], // Keep last 20 logs
    }))
  }

  const setGoogleStatus = (status: ServiceStatus) => {
    setState((s) => ({ ...s, googleStatus: status }))
  }


  // Load mock logs only if no real logs are present
  useEffect(() => {
    if (state.user && state.activityLogs.length === 0) {
      setState((s) => ({ ...s, isLogsLoading: true }))
      setTimeout(() => {
        setState((s) => {
          // Only set mock logs if still no real logs present
          if (s.activityLogs.length === 0) {
            return { ...s, activityLogs: mockLogs, isLogsLoading: false }
          }
          return { ...s, isLogsLoading: false }
        })
      }, 1000)
    }
  }, [state.user])

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
  }

  return <AppStateContext.Provider value={{ state, actions }}>{children}</AppStateContext.Provider>
}

export function useAppState() {
  const context = useContext(AppStateContext)
  if (context === undefined) {
    throw new Error("useAppState must be used within an AppStateProvider")
  }
  return context
}
