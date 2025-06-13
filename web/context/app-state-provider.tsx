"use client"

import type React from "react"
import { createContext, useContext, useState, useEffect } from "react"
import { useRouter, usePathname } from "next/navigation"
import type { LogEntry } from "@/components/activity-log" // Assuming LogEntry type is in ActivityLog

interface User {
  email: string
  name: string
  avatarUrl?: string
}

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
  connectStrava: () => void
  disconnectStrava: () => void
  reauthorizeStrava: () => void
  saveSpreadsheet: (url: string) => void
  changeSpreadsheet: () => void
  triggerManualSync: () => void
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

  // Authentication
  useEffect(() => {
    // Simulate checking stored auth state
    const storedUser = localStorage.getItem("appUser")
    if (storedUser) {
      setState((s) => ({ ...s, user: JSON.parse(storedUser), isAuthLoading: false }))
    } else {
      setState((s) => ({ ...s, isAuthLoading: false }))
    }
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
    await new Promise((resolve) => setTimeout(resolve, 1000)) // Simulate API call
    const mockUser: User = {
      email: "user@example.com",
      name: "Demo User",
      avatarUrl: "/placeholder.svg?width=40&height=40",
    }
    localStorage.setItem("appUser", JSON.stringify(mockUser))
    setState((s) => ({
      ...s,
      user: mockUser,
      isAuthLoading: false,
      googleStatus: "Connected", // Google connected by default on sign-in
      // Reset other states to initial for new session as per flow
      stravaStatus: "NotConnected",
      stravaUserName: undefined,
      stravaAvatarUrl: undefined,
      spreadsheetStatus: "Disabled",
      spreadsheetUrl: undefined,
      manualSyncStatus: "Disabled",
    }))
    router.push("/dashboard")
  }

  const signOut = () => {
    localStorage.removeItem("appUser")
    setState((s) => ({ ...s, user: null }))
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

  // Load mock logs
  useEffect(() => {
    if (state.user) {
      setState((s) => ({ ...s, isLogsLoading: true }))
      setTimeout(() => {
        setState((s) => ({ ...s, activityLogs: mockLogs, isLogsLoading: false }))
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
