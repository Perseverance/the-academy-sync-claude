"use client"

import { useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { RefreshCw, Loader2, Info, CheckCircle, AlertCircle } from "lucide-react"
import type { SyncStatus } from "@/context/app-state-provider"
import { SyncService, SyncError } from "@/src/services/SyncService"

interface ManualSyncCardProps {
  status: SyncStatus
  onSync?: () => void // Made optional since we handle sync internally now
}

export function ManualSyncCard({ status, onSync }: ManualSyncCardProps) {
  const [isProcessing, setIsProcessing] = useState(false)
  const [lastSyncResult, setLastSyncResult] = useState<{
    success: boolean
    message: string
    traceId?: string
  } | null>(null)

  const isDisabled = status === "Disabled" || isProcessing

  const buttonText = isProcessing ? "Syncing..." : "Sync Now"
  const ButtonIcon = isProcessing ? Loader2 : RefreshCw

  const disabledTooltipText = "Complete Strava connection and Spreadsheet configuration to enable manual sync."

  const handleSync = async () => {
    if (isDisabled) return

    setIsProcessing(true)
    setLastSyncResult(null)

    try {
      const response = await SyncService.triggerManualSync()
      
      setLastSyncResult({
        success: true,
        message: "Sync triggered successfully! Your data will be updated shortly.",
        traceId: response.trace_id
      })

      // Call the optional onSync callback if provided
      if (onSync) {
        onSync()
      }
    } catch (error: unknown) {
      console.error('Manual sync failed:', error)
      
      let errorMessage = "Failed to trigger sync. Please try again."
      
      if (error instanceof SyncError) {
        switch (error.code) {
          case 'USER_NOT_CONFIGURED':
            errorMessage = "Please complete your OAuth connections and spreadsheet setup first."
            break
          case 'SERVICE_UNAVAILABLE':
            errorMessage = "Sync service is temporarily unavailable. Please try again later."
            break
          case 'UNAUTHORIZED':
            errorMessage = "Your session has expired. Please sign in again."
            break
          default:
            errorMessage = error.message || errorMessage
        }
      } else if (error instanceof Error) {
        // Handle standard Error instances
        errorMessage = error.message || errorMessage
      } else if (typeof error === 'string') {
        // Handle string errors
        errorMessage = error
      }

      setLastSyncResult({
        success: false,
        message: errorMessage
      })
    } finally {
      setIsProcessing(false)
    }
  }

  const renderButton = () => (
    <button 
      onClick={handleSync} 
      disabled={isDisabled} 
      className="btn-primary-main w-full"
    >
      <ButtonIcon className={`h-5 w-5 ${isProcessing ? "animate-spin" : ""}`} />
      {buttonText}
    </button>
  )

  return (
    <Card className="shadow-md">
      <CardHeader>
        <CardTitle className="text-xl flex items-center gap-2">
          <RefreshCw className="h-6 w-6" />
          On-Demand Sync
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {/* Sync Button */}
        {status === "Disabled" ? (
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

        {/* Status Messages */}
        {status === "Disabled" && (
          <p className="text-xs text-muted-foreground flex items-center gap-1">
            <Info size={14} /> {disabledTooltipText}
          </p>
        )}

        {/* Sync Result Feedback */}
        {lastSyncResult && (
          <div className={`p-3 rounded-lg border ${
            lastSyncResult.success 
              ? 'bg-green-50 border-green-200 text-green-800' 
              : 'bg-red-50 border-red-200 text-red-800'
          }`}>
            <div className="flex items-start gap-2">
              {lastSyncResult.success ? (
                <CheckCircle className="h-4 w-4 mt-0.5 flex-shrink-0" />
              ) : (
                <AlertCircle className="h-4 w-4 mt-0.5 flex-shrink-0" />
              )}
              <div className="flex-1">
                <p className="text-sm font-medium">
                  {lastSyncResult.message}
                </p>
                {lastSyncResult.traceId && (
                  <p className="text-xs mt-1 font-mono">
                    Trace ID: {lastSyncResult.traceId}
                  </p>
                )}
              </div>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
