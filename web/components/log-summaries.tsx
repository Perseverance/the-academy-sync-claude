"use client"

import { useEffect, useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { useAuth } from "@/components/auth-provider"
import { CheckCircle, XCircle, AlertTriangle, Loader2, ListChecks } from "lucide-react"

interface LogEntry {
  id: string
  date: string
  status: "success" | "failure" | "partial_error"
  message: string
}

const mockLogEntries: LogEntry[] = [
  {
    id: "1",
    date: "2025-05-28",
    status: "success",
    message: "Successfully logged 10.50 km and 00:55:30. Workout description in 'Описание на тренировката' updated.",
  },
  {
    id: "2",
    date: "2025-05-27",
    status: "failure",
    message: "Failed to update entry. Reason: Strava API unavailable after multiple retries.",
  },
  {
    id: "3",
    date: "2025-05-26",
    status: "success",
    message: "Successfully logged 8.20 km and 00:45:10. Workout description in 'Описание на тренировката' updated.",
  },
  {
    id: "4",
    date: "2025-05-25",
    status: "partial_error",
    message: "Logged 5.0 km but failed to update 'Описание на тренировката' due to Google Sheets API error.",
  },
  {
    id: "5",
    date: "2025-05-24",
    status: "success",
    message: "Successfully logged 12.10 km and 01:02:00. Workout description in 'Описание на тренировката' updated.",
  },
]

export function LogSummaries() {
  const { user, isLoading: authLoading } = useAuth()
  const [logEntries, setLogEntries] = useState<LogEntry[]>([])
  const [isFetchingLogs, setIsFetchingLogs] = useState(true)

  useEffect(() => {
    if (user) {
      const fetchLogs = async () => {
        setIsFetchingLogs(true)
        await new Promise((resolve) => setTimeout(resolve, 1000)) // Simulate API delay
        // In a real app, fetch last 30 days of logs from backend
        setLogEntries(mockLogEntries.slice(0, 30)) // Show up to 30
        setIsFetchingLogs(false)
      }
      fetchLogs()
    }
  }, [user])

  if (authLoading || isFetchingLogs) {
    return (
      <div className="flex items-center justify-center h-[calc(100vh-150px)]">
        <Loader2 className="h-10 w-10 animate-spin text-primary" />
      </div>
    )
  }

  const getStatusDisplay = (status: LogEntry["status"]) => {
    switch (status) {
      case "success":
        return {
          icon: <CheckCircle className="w-5 h-5 text-success flex-shrink-0" />,
          badge: <Badge className="bg-success/20 text-success border-success/30">Success</Badge>,
        }
      case "failure":
        return {
          icon: <XCircle className="w-5 h-5 text-destructive flex-shrink-0" />,
          badge: <Badge variant="destructive">Failure</Badge>,
        }
      case "partial_error":
        return {
          icon: <AlertTriangle className="w-5 h-5 text-orange-400 flex-shrink-0" />,
          badge: <Badge className="bg-orange-500/20 text-orange-400 border-orange-500/30">Partial Error</Badge>,
        }
      default:
        return {
          icon: <AlertTriangle className="w-5 h-5 text-muted-foreground flex-shrink-0" />,
          badge: <Badge variant="secondary">Unknown</Badge>,
        }
    }
  }

  return (
    <div className="max-w-3xl mx-auto">
      <h2 className="text-3xl font-bold mb-6 text-foreground flex items-center">
        <ListChecks className="mr-3 h-8 w-8 text-primary" />
        My Automation Log Summaries
      </h2>
      <Card>
        <CardHeader>
          <CardTitle>Processing History (Last 30 Days)</CardTitle>
        </CardHeader>
        <CardContent>
          {logEntries.length > 0 ? (
            <ul className="space-y-4">
              {logEntries.map((entry) => {
                const { icon, badge } = getStatusDisplay(entry.status)
                return (
                  <li
                    key={entry.id}
                    className="p-4 border border-border rounded-lg bg-card hover:bg-muted/30 transition-colors"
                  >
                    <div className="flex items-start space-x-3">
                      {icon}
                      <div className="flex-1">
                        <div className="flex justify-between items-center mb-1">
                          <p className="text-sm font-medium text-foreground">
                            {new Date(entry.date).toLocaleDateString("en-US", {
                              year: "numeric",
                              month: "long",
                              day: "numeric",
                            })}
                          </p>
                          {badge}
                        </div>
                        <p className="text-sm text-muted-foreground">{entry.message}</p>
                      </div>
                    </div>
                  </li>
                )
              })}
            </ul>
          ) : (
            <div className="text-center py-10">
              <ListChecks className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
              <p className="text-muted-foreground">No log entries found for the last 30 days.</p>
              <p className="text-sm text-muted-foreground">Once the automation runs, summaries will appear here.</p>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
