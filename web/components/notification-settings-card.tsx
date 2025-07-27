"use client"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Bell, Mail, Bot } from "lucide-react"
import { useAppState } from "@/context/app-state-provider"
import { useState } from "react"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"

export function NotificationSettingsCard() {
  const { state, actions } = useAppState()
  const [isUpdating, setIsUpdating] = useState(false)
  const { t } = useTranslation()

  if (!state.user) return null

  const canEnableAutomation = state.user.has_strava_connection && state.user.has_sheets_connection

  const handleAutomationToggle = async (checked: boolean) => {
    if (!canEnableAutomation && checked) {
      toast.error(t('settings.toast.cannotEnableAutomation.title'), {
        description: t('settings.toast.cannotEnableAutomation.description')
      })
      return
    }

    setIsUpdating(true)
    try {
      await actions.updateUserSettings({
        automation_enabled: checked,
        email_notifications_enabled: state.user.email_notifications_enabled
      })
      toast.success(checked ? t('settings.toast.automationEnabled') : t('settings.toast.automationDisabled'))
    } catch (error: any) {
      toast.error(t('settings.toast.failedToUpdate.title'), {
        description: error.message || t('settings.toast.failedToUpdate.description')
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
      toast.success(checked ? t('settings.toast.notificationsEnabled') : t('settings.toast.notificationsDisabled'))
    } catch (error: any) {
      toast.error(t('settings.toast.failedToUpdate.title'), {
        description: error.message || t('settings.toast.failedToUpdate.description')
      })
    } finally {
      setIsUpdating(false)
    }
  }

  return (
    <Card className="border-border/50 shadow-lg shadow-background/5">
      <CardHeader className="space-y-1.5">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10">
            <Bell className="h-5 w-5 text-primary" />
          </div>
          <div>
            <CardTitle className="text-xl">{t('settings.notifications.title')}</CardTitle>
            <CardDescription className="text-sm">
              {t('settings.notifications.subtitle')}
            </CardDescription>
          </div>
        </div>
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
                {t('settings.automation.label')}
              </Label>
              <p className="text-sm text-muted-foreground">
                {canEnableAutomation 
                  ? t('settings.automation.enabled')
                  : t('settings.automation.disabled')}
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
                {t('settings.notifications.label')}
              </Label>
              <p className="text-sm text-muted-foreground">
                {t('settings.notifications.description')}
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