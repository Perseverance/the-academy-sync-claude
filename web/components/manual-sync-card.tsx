"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { RefreshCw, Loader2, Info } from "lucide-react"
import type { SyncStatus } from "@/context/app-state-provider"
import { useTranslation } from "react-i18next"

interface ManualSyncCardProps {
  status: SyncStatus
  onSync: () => void
}

export function ManualSyncCard({ status, onSync }: ManualSyncCardProps) {
  const { t } = useTranslation()
  const isDisabled = status === "Disabled"
  const isProcessing = status === "Processing"

  const buttonText = isProcessing ? t('manualSync.actions.syncing') : t('manualSync.actions.sync')
  const ButtonIcon = isProcessing ? Loader2 : RefreshCw

  const disabledTooltipText = t('manualSync.tooltip')

  const renderButton = () => (
    <button onClick={onSync} disabled={isDisabled || isProcessing} className="btn-primary-main w-full">
      <ButtonIcon className={`h-5 w-5 ${isProcessing ? "animate-spin" : ""}`} />
      {buttonText}
    </button>
  )

  return (
    <Card className="shadow-md">
      <CardHeader>
        <CardTitle className="text-xl flex items-center gap-2">
          <RefreshCw className="h-6 w-6" />
          {t('manualSync.title')}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {isDisabled ? (
          <TooltipProvider>
            <Tooltip delayDuration={100}>
              <TooltipTrigger asChild className="w-full">{renderButton()}</TooltipTrigger>
              <TooltipContent className="bg-foreground text-background">
                <p>{disabledTooltipText}</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        ) : (
          renderButton()
        )}
        {isDisabled && (
          <p className="text-xs text-muted-foreground mt-2 flex items-center gap-1">
            <Info size={14} /> {disabledTooltipText}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
