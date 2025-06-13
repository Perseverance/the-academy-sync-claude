"use client"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { ScrollArea } from "@/components/ui/scroll-area"
import { ListChecks, CheckCircle, XCircle, AlertTriangle, Loader2 } from "lucide-react"

export interface LogEntry {
  id: string
  date: string
  status: "Success" | "Failure" | "SuccessWithWarning"
  summary: string
}

interface ActivityLogProps {
  logs: LogEntry[]
  isLoading: boolean
}

function LogItem({ log }: { log: LogEntry }) {
  const getStatusAttributes = () => {
    switch (log.status) {
      case "Success":
        return {
          icon: <CheckCircle className="h-4 w-4 text-success" />,
          badgeVariant: "default",
          badgeClass: "bg-success/20 text-success border-success/30",
        }
      case "Failure":
        return { icon: <XCircle className="h-4 w-4 text-destructive" />, badgeVariant: "destructive", badgeClass: "" }
      case "SuccessWithWarning":
        return {
          icon: <AlertTriangle className="h-4 w-4 text-warning" />,
          badgeVariant: "outline",
          badgeClass: "bg-warning/10 text-warning-foreground border-warning/30",
        }
      default:
        return {
          icon: <AlertTriangle className="h-4 w-4 text-muted-foreground" />,
          badgeVariant: "secondary",
          badgeClass: "",
        }
    }
  }

  const { icon, badgeVariant, badgeClass } = getStatusAttributes()
  const formattedDate = new Date(log.date).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })

  return (
    <li className="py-3 px-1 border-b border-border last:border-b-0">
      <div className="flex items-start space-x-3">
        <div className="flex-shrink-0 pt-0.5">{icon}</div>
        <div className="flex-1 space-y-0.5">
          <div className="flex justify-between items-center">
            <p className="text-xs font-medium text-muted-foreground">{formattedDate}</p>
            <Badge variant={badgeVariant} className={`text-xs ${badgeClass}`}>
              {log.status.replace(/([A-Z])/g, " $1").trim()}
            </Badge>
          </div>
          <p className="text-sm text-foreground/90">{log.summary}</p>
        </div>
      </div>
    </li>
  )
}

export function ActivityLog({ logs, isLoading }: ActivityLogProps) {
  return (
    <Card className="shadow-md h-full flex flex-col">
      {" "}
      {/* Ensure card can grow */}
      <CardHeader>
        <CardTitle className="text-xl flex items-center gap-2">
          <ListChecks className="h-6 w-6" />
          Recent Automation Activity
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
            <p>No automation activity found for the last 30 days.</p>
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
