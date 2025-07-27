"use client"

import { useState } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { FileSpreadsheet, Edit3, Save, Info, Loader2, CheckCircle } from "lucide-react"
import type { SpreadsheetConfigStatus } from "@/context/app-state-provider"
import { useToast } from "@/hooks/use-toast"
import { configService, ConfigApiError } from "@/services/config"
import { Badge } from "@/components/ui/badge"
import { useTranslation } from "react-i18next"

interface SpreadsheetCardProps {
  status: SpreadsheetConfigStatus
  configuredUrl?: string
  onSave: (url: string) => Promise<void>
  onChange: () => void
}

export function SpreadsheetCard({ status, configuredUrl, onSave, onChange }: SpreadsheetCardProps) {
  const [urlInput, setUrlInput] = useState(configuredUrl || "")
  const [isLoading, setIsLoading] = useState(false)
  const { toast } = useToast()
  const { t } = useTranslation()

  const handleSave = async () => {
    // Basic client-side validation
    if (!urlInput.trim()) {
      toast({
        title: t('spreadsheetCard.toast.emptyUrl.title'),
        description: t('spreadsheetCard.toast.emptyUrl.description'),
        variant: "destructive",
      })
      return
    }

    if (!urlInput.startsWith("https://docs.google.com/spreadsheets/d/")) {
      toast({
        title: t('spreadsheetCard.toast.invalidUrl.title'),
        description: t('spreadsheetCard.toast.invalidUrl.description'),
        variant: "destructive",
      })
      return
    }

    setIsLoading(true)
    try {
      await onSave(urlInput)
      toast({
        title: t('spreadsheetCard.toast.success.title'),
        description: t('spreadsheetCard.toast.success.description'),
      })
    } catch (error) {
      console.error("Error saving spreadsheet:", error)
      
      // Handle ConfigApiError with user-friendly messages
      if (error instanceof ConfigApiError) {
        toast({
          title: t('spreadsheetCard.toast.configError.title'),
          description: error.getUserFriendlyMessage(),
          variant: "destructive",
        })
      } else {
        toast({
          title: t('spreadsheetCard.toast.error.title'),
          description: t('spreadsheetCard.toast.error.description'),
          variant: "destructive",
        })
      }
    } finally {
      setIsLoading(false)
    }
  }

  const isDisabled = status === "Disabled"
  const isConfigured = status === "Configured"

  return (
    <Card className="shadow-md">
      <CardHeader>
        <CardTitle className="text-xl flex items-center gap-2">
          <FileSpreadsheet className="h-6 w-6" />
          {t('spreadsheetCard.title')}
          {isConfigured && (
            <Badge className="bg-success/20 text-success border-success/30 ml-auto">
              <CheckCircle className="h-3 w-3 mr-1" /> {t('spreadsheetCard.status')}
            </Badge>
          )}
        </CardTitle>
        {isConfigured && configuredUrl && (
          <CardDescription className="text-sm pt-1 block overflow-hidden">
            <a
              href={configuredUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="link-main truncate inline-block max-w-[calc(100%-theme(spacing.28))]"
              title={configuredUrl} // Show full URL on hover
            >
              {configuredUrl}
            </a>
          </CardDescription>
        )}
      </CardHeader>
      <CardContent className="space-y-4">
        {isDisabled && (
          <div className="flex items-center space-x-2 text-muted-foreground bg-muted/50 p-3 rounded-md border border-dashed">
            <Info className="h-5 w-5" />
            <p className="text-sm">{t('spreadsheetCard.messages.connectStravaFirst')}</p>
          </div>
        )}

        {!isConfigured && !isDisabled && (
          <div>
            <Label htmlFor="spreadsheet-url" className="text-sm font-semibold text-foreground/90">
              {t('spreadsheetCard.label')}
            </Label>
            <div className="flex items-center space-x-2 mt-1">
              <Input
                id="spreadsheet-url"
                type="url"
                placeholder={t('spreadsheetCard.placeholder')}
                value={urlInput}
                onChange={(e) => setUrlInput(e.target.value)}
                disabled={isDisabled}
                className="input-main"
              />
              <button 
                onClick={handleSave} 
                className="btn-primary-main px-4" 
                disabled={isDisabled || !urlInput.trim() || isLoading}
              >
                {isLoading ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Save className="h-4 w-4" />
                )}
                {isLoading ? t('spreadsheetCard.actions.saving') : t('spreadsheetCard.actions.save')}
              </button>
            </div>
          </div>
        )}

        {isConfigured && (
          <button onClick={onChange} className="btn-secondary-main w-full text-sm py-1.5 px-3">
            <Edit3 className="h-4 w-4" />
            {t('spreadsheetCard.actions.change')}
          </button>
        )}
      </CardContent>
    </Card>
  )
}
