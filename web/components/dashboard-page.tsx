"use client"

import { useAppState } from "@/context/app-state-provider"
import { ConnectionCard } from "@/components/connection-card"
import { SpreadsheetCard } from "@/components/spreadsheet-card"
import { ManualSyncCard } from "@/components/manual-sync-card"
import { ActivityLog } from "@/components/activity-log"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { GoogleLogo } from "@/components/icons/google-logo"
import { StravaLogo } from "@/components/icons/strava-logo" // Import new StravaLogo
import { LogOut, Settings } from "lucide-react"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Button } from "@/components/ui/button" // shadcn button for dropdown trigger
import { AcademyLogo } from "./icons/academy-logo"

export function DashboardPage() {
  const { state, actions } = useAppState()

  if (!state.user) {
    // This should be handled by AppStateProvider redirect, but as a fallback
    return null
  }

  return (
    <div className="min-h-screen flex flex-col">
      {/* Header */}
      <header className="bg-card border-b border-border shadow-sm sticky top-0 z-30">
        <div className="container mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <AcademyLogo className="h-8 w-8" />
            <h1 className="text-2xl font-brand text-primary">Configuration</h1>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" className="relative h-10 w-10 rounded-full p-0">
                <Avatar className="h-9 w-9">
                  <AvatarImage src={state.user.avatarUrl || "/placeholder.svg"} alt={state.user.name} />
                  <AvatarFallback>{state.user.name.charAt(0)}</AvatarFallback>
                </Avatar>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent className="w-56" align="end" forceMount>
              <DropdownMenuLabel className="font-normal">
                <div className="flex flex-col space-y-1">
                  <p className="text-sm font-medium leading-none">{state.user.name}</p>
                  <p className="text-xs leading-none text-muted-foreground">{state.user.email}</p>
                </div>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem disabled>
                <Settings className="mr-2 h-4 w-4" />
                <span>Settings</span>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={actions.signOut}>
                <LogOut className="mr-2 h-4 w-4" />
                <span>Log out</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1 bg-background p-4 sm:p-6 lg:p-8">
        <div className="max-w-5xl mx-auto">
          <h2 className="text-3xl font-brand font-bold text-primary mb-6">Configuration Dashboard</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {/* Google Connection Card */}
            <ConnectionCard
              serviceName="Google Account"
              serviceIcon={<GoogleLogo className="h-8 w-8 text-primary" />} // Google logo can use primary color or its own
              status={state.googleStatus}
              userName={state.user.email} // Using email as username for Google
              userAvatarUrl={state.user.avatarUrl}
              onConnect={() => {
                /* Google is connected by sign-in */
              }}
              onDisconnect={() => {
                /* Typically don't disconnect Google session this way */ actions.signOut()
              }}
              onReauthorize={() => {
                /* Implement Google re-auth if needed */
              }}
              isGoogle={true}
            />

            {/* Strava Connection Card */}
            <ConnectionCard
              serviceName="Strava"
              serviceIcon={<StravaLogo className="h-8 w-8" />} // Use StravaLogo component
              status={state.stravaStatus}
              userName={state.stravaUserName}
              userAvatarUrl={state.stravaAvatarUrl}
              onConnect={actions.connectStrava}
              onDisconnect={actions.disconnectStrava}
              onReauthorize={actions.reauthorizeStrava}
            />

            {/* Spreadsheet Card - Spans 1 column, or more if fewer items */}
            <SpreadsheetCard
              status={state.spreadsheetStatus}
              configuredUrl={state.spreadsheetUrl}
              onSave={actions.saveSpreadsheet}
              onChange={actions.changeSpreadsheet}
            />

            {/* Manual Sync Card - Spans 1 column */}
            <ManualSyncCard status={state.manualSyncStatus} onSync={actions.triggerManualSync} />

            {/* Activity Log - Spans full width on its row or multiple columns */}
            <div className="md:col-span-2 lg:col-span-3">
              <ActivityLog logs={state.activityLogs} isLoading={state.isLogsLoading} />
            </div>
          </div>
        </div>
      </main>
    </div>
  )
}
