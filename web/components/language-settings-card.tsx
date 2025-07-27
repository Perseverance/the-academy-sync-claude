"use client"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Globe } from "lucide-react"
import { useAppState } from "@/context/app-state-provider"
import { useState } from "react"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"

export function LanguageSettingsCard() {
  const { state, actions } = useAppState()
  const [isUpdating, setIsUpdating] = useState(false)
  const { t, i18n } = useTranslation()

  if (!state.user) return null

  const handleLanguageChange = async (newLanguage: string) => {
    setIsUpdating(true)
    try {
      await actions.updateUserSettings({
        automation_enabled: state.user.automation_enabled,
        email_notifications_enabled: state.user.email_notifications_enabled,
        language_preference: newLanguage
      })
      
      // Change the UI language immediately
      await i18n.changeLanguage(newLanguage)
      
      toast.success(t('settings.toast.languageUpdated'))
    } catch (error: any) {
      console.error('Failed to update language:', error)
      toast.error(t('settings.toast.failedToUpdate.title'), {
        description: t('settings.toast.failedToUpdate.description')
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
            <Globe className="h-5 w-5 text-primary" />
          </div>
          <div>
            <CardTitle className="text-xl">{t('settings.language.title')}</CardTitle>
            <CardDescription className="text-sm">
              {t('settings.language.description')}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="language">{t('settings.language.label')}</Label>
          <Select
            value={state.user.language_preference || 'bg'}
            onValueChange={handleLanguageChange}
            disabled={isUpdating}
          >
            <SelectTrigger id="language" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="bg">{t('settings.language.options.bulgarian')}</SelectItem>
              <SelectItem value="en">{t('settings.language.options.english')}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </CardContent>
    </Card>
  )
}