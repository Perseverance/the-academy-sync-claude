"use client"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Settings, Mail, Bot } from "lucide-react"
import { useAppState } from "@/context/app-state-provider"
import { useState } from "react"
import { toast } from "sonner"

export function SettingsCard() {
  const { state, actions } = useAppState()
  const [isUpdating, setIsUpdating] = useState(false)

  if (!state.user) return null

  const canEnableAutomation = state.user.has_strava_connection && state.user.has_sheets_connection

  const handleAutomationToggle = async (checked: boolean) => {
    if (!canEnableAutomation && checked) {
      toast.error("Cannot enable automation", {
        description: "Please connect both Strava and Google Sheets first"
      })
      return
    }

    setIsUpdating(true)
    try {
      await actions.updateUserSettings({
        automation_enabled: checked,
        email_notifications_enabled: state.user.email_notifications_enabled
      })
      toast.success(checked ? "Automation enabled" : "Automation disabled")
    } catch (error: any) {
      toast.error("Failed to update settings", {
        description: error.message || "Please try again"
      })
    } finally {
      setIsUpdating(false)
    }
  }

  const handleEmailToggle = async (checked: boolean) => {
    setIsUpdating(true)
    try {
      await actions.updateUserSettings({
        automation_enabled: state.user.automation_enabled,
        email_notifications_enabled: checked
      })
      toast.success(checked ? "Email notifications enabled" : "Email notifications disabled")
    } catch (error: any) {
      toast.error("Failed to update settings", {
        description: error.message || "Please try again"
      })
    } finally {
      setIsUpdating(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Settings className="h-5 w-5" />
          Settings
        </CardTitle>
        <CardDescription>
          Manage your automation and notification preferences
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="flex items-center justify-between space-x-2">
          <div className="flex items-center space-x-3">
            <Bot className="h-5 w-5 text-muted-foreground" />
            <div className="space-y-1">
              <Label 
                htmlFor="automation-toggle" 
                className={!canEnableAutomation ? "text-muted-foreground" : ""}
              >
                Daily Automation
              </Label>
              <p className="text-sm text-muted-foreground">
                {canEnableAutomation 
                  ? "Automatically sync activities to your spreadsheet daily"
                  : "Connect Strava and Google Sheets to enable automation"}
              </p>
            </div>
          </div>
          <Switch
            id="automation-toggle"
            checked={state.user.automation_enabled}
            onCheckedChange={handleAutomationToggle}
            disabled={!canEnableAutomation || isUpdating}
          />
        </div>

        <div className="flex items-center justify-between space-x-2">
          <div className="flex items-center space-x-3">
            <Mail className="h-5 w-5 text-muted-foreground" />
            <div className="space-y-1">
              <Label htmlFor="email-toggle">
                Email Notifications
              </Label>
              <p className="text-sm text-muted-foreground">
                Receive email updates about sync status
              </p>
            </div>
          </div>
          <Switch
            id="email-toggle"
            checked={state.user.email_notifications_enabled}
            onCheckedChange={handleEmailToggle}
            disabled={isUpdating}
          />
        </div>
      </CardContent>
    </Card>
  )
}