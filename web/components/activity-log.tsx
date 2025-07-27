"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { ListChecks, CheckCircle, XCircle, AlertTriangle, Loader2 } from "lucide-react"
import { useTranslation } from "react-i18next"

export interface LogEntry {
  id: string
  date: string
  status: "Success" | "Failure" | "SuccessWithWarning"
  summary: string
  metadata?: {
    messageType?: string
    errorMessage?: string
    date?: string
    activitiesFound?: number
    activitiesProcessed?: number
    spreadsheetUpdated?: boolean
    reason?: string
    activityCount?: number
    distance?: number
    duration?: string
  }
}

interface ActivityLogProps {
  logs: LogEntry[]
  isLoading: boolean
}

function LogItem({ log }: { log: LogEntry }) {
  const { t, i18n } = useTranslation()
  
  const getStatusAttributes = () => {
    switch (log.status) {
      case "Success":
        return {
          icon: <CheckCircle className="h-4 w-4 text-success" />,
          badgeVariant: "default" as const,
          badgeClass: "bg-success/20 text-success border-success/30",
        }
      case "Failure":
        return { icon: <XCircle className="h-4 w-4 text-destructive" />, badgeVariant: "destructive" as const, badgeClass: "" }
      case "SuccessWithWarning":
        return {
          icon: <AlertTriangle className="h-4 w-4 text-warning" />,
          badgeVariant: "outline" as const,
          badgeClass: "bg-warning/10 text-warning-foreground border-warning/30",
        }
      default:
        return {
          icon: <AlertTriangle className="h-4 w-4 text-muted-foreground" />,
          badgeVariant: "secondary" as const,
          badgeClass: "",
        }
    }
  }

  const { icon, badgeVariant, badgeClass } = getStatusAttributes()
  const formattedDate = new Date(log.date).toLocaleDateString(i18n.language === 'bg' ? 'bg-BG' : 'en-US', {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })

  // Generate localized summary based on metadata
  const getLocalizedSummary = () => {
    if (!log.metadata || !log.metadata.messageType) {
      return log.summary // Fallback to English summary
    }

    const { messageType, ...params } = log.metadata

    switch (messageType) {
      case "error":
        return params.errorMessage || log.summary
      
      case "failed":
        return t('activityLog.messages.processingFailed')
      
      case "skipped":
        return params.reason || t('activityLog.messages.processingSkipped')
      
      case "processed":
        if (params.activitiesFound === 0) {
          return t('activityLog.messages.noActivitiesFound')
        } else if (params.activitiesFound === 1) {
          return t('activityLog.messages.activityFound')
        } else {
          return t('activityLog.messages.activitiesFound', { count: params.activitiesFound })
        }
      
      case "noTrainingPlan":
        return t('activityLog.messages.noTrainingPlan')
      
      case "alreadyProcessed":
        return t('activityLog.messages.alreadyProcessed')
      
      case "noActivities":
        return t('activityLog.messages.noActivities')
      
      case "restDayNoActivity":
        return t('activityLog.messages.restDayNoActivity')
      
      case "noScheduledTraining":
        return t('activityLog.messages.noScheduledTraining')
      
      case "noActivitiesFound":
        return t('activityLog.messages.noActivitiesFound')
      
      case "activityLogged":
        return t('activityLog.messages.activityLogged', {
          distance: params.distance?.toFixed(1),
          duration: params.duration
        })
      
      case "activitiesLogged":
        return t('activityLog.messages.activitiesLogged', {
          count: params.activityCount,
          distance: params.distance?.toFixed(1),
          duration: params.duration
        })
      
      default:
        return log.summary
    }
  }

  const localizedSummary = getLocalizedSummary()

  return (
    <li className="py-3 px-1 border-b border-border last:border-b-0">
      <div className="flex items-start space-x-3">
        <div className="flex-shrink-0 pt-0.5">{icon}</div>
        <div className="flex-1 space-y-0.5">
          <div className="flex justify-between items-center">
            <p className="text-xs font-medium text-muted-foreground">{formattedDate}</p>
            <Badge variant={badgeVariant} className={`text-xs ${badgeClass}`}>
              {t(`activityLog.status.${log.status.charAt(0).toLowerCase()}${log.status.slice(1)}`)}
            </Badge>
          </div>
          <p className="text-sm text-foreground/90">{localizedSummary}</p>
        </div>
      </div>
    </li>
  )
}

export function ActivityLog({ logs, isLoading }: ActivityLogProps) {
  const { t } = useTranslation()
  
  return (
    <Card className="shadow-md h-full flex flex-col">
      {" "}
      {/* Ensure card can grow */}
      <CardHeader>
        <CardTitle className="text-xl flex items-center gap-2">
          <ListChecks className="h-6 w-6" />
          {t('activityLog.title')}
        </CardTitle>
      </CardHeader>
      <CardContent className="flex-grow overflow-hidden">
        {" "}
        {/* Allow content to take space and hide overflow for ScrollArea */}
        {isLoading ? (
          <div className="flex items-center justify-center h-40">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
          </div>
        ) : logs.length === 0 ? (
          <div className="text-center py-10 text-muted-foreground">
            <ListChecks className="mx-auto h-10 w-10 mb-3" />
            <p>{t('activityLog.noActivity')}</p>
          </div>
        ) : (
          <ScrollArea className="h-[300px] pr-3">
            {" "}
            {/* Set a fixed or max height for scroll area */}
            <ul className="space-y-0">
              {logs.map((log) => (
                <LogItem key={log.id} log={log} />
              ))}
            </ul>
          </ScrollArea>
        )}
      </CardContent>
    </Card>
  )
}
