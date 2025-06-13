"use client"

import { useEffect, useState } from "react"
// import { Button } from "@/components/ui/button"; // Using custom buttons now
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input" // Will use shadcn input and style via globals/tailwind.config
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { useAuth } from "@/components/auth-provider"
import { useToast } from "@/hooks/use-toast"
import {
  CheckCircle,
  XCircle,
  AlertTriangle,
  ExternalLink,
  RefreshCw,
  UserCircle2,
  FileSpreadsheet,
  Activity,
  Clock,
  Loader2,
  LinkIcon,
  Unlink,
} from "lucide-react"

// ... (interfaces remain the same) ...
interface ConnectionStatus {
  google: "active" | "needs_reauth" | "error"
  strava: "connected" | "not_connected" | "needs_reauth"
  stravaUsername?: string
}

interface LastRunStatus {
  date: string
  status: "success" | "error" | "partial_error"
  message: string
}

export function Dashboard() {
  const { user, isLoading: authLoading } = useAuth()
  const { toast } = useToast()

  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>({
    google: "active",
    strava: "not_connected",
  })
  const [spreadsheetUrl, setSpreadsheetUrl] = useState("")
  const [inputSpreadsheetUrl, setInputSpreadsheetUrl] = useState("")

  const [lastRunStatus, setLastRunStatus] = useState<LastRunStatus | null>(null)
  const [isFetchingData, setIsFetchingData] = useState(true)
  const [isSavingUrl, setIsSavingUrl] = useState(false)
  const [isConnectingStrava, setIsConnectingStrava] = useState(false)
  const [isDisconnectingStrava, setIsDisconnectingStrava] = useState(false)
  const [isReauthorizingGoogle, setIsReauthorizingGoogle] = useState(false)

  useEffect(() => {
    if (user) {
      const fetchData = async () => {
        setIsFetchingData(true)
        await new Promise((resolve) => setTimeout(resolve, 1200))
        setConnectionStatus({
          google: "active", // Mocked
          strava: "connected", // Mocked
          stravaUsername: "AcademyRunner", // Mocked
        })
        const savedUrl = "https://docs.google.com/spreadsheets/d/example123/edit" // Mocked
        setSpreadsheetUrl(savedUrl)
        setInputSpreadsheetUrl(savedUrl)
        setLastRunStatus({
          date: new Date().toLocaleDateString("en-CA"),
          status: "success",
          message:
            "Successfully logged 10.50 km and 00:55:30. Workout description in 'Описание на тренировката' updated.",
        })
        setIsFetchingData(false)
      }
      fetchData()
    }
  }, [user])

  const initiateStravaAuth = (isReauth = false) => {
    const stravaClientId = "YOUR_STRAVA_CLIENT_ID"
    const frontendCallbackUrl = `${window.location.origin}/auth/strava/callback`
    const scopes = "activity:read_all,profile:read_all"
    localStorage.setItem("stravaOAuthRedirect", "/dashboard")
    const params = new URLSearchParams({
      client_id: stravaClientId,
      redirect_uri: frontendCallbackUrl,
      response_type: "code",
      approval_prompt: isReauth ? "force" : "auto",
      scope: scopes,
    })
    window.location.href = `https://www.strava.com/oauth/authorize?${params.toString()}`
  }

  const handleConnectStrava = () => {
    setIsConnectingStrava(true)
    initiateStravaAuth(false)
  }
  const handleReauthorizeStrava = () => {
    setIsConnectingStrava(true)
    initiateStravaAuth(true)
  }

  const handleDisconnectStrava = async () => {
    setIsDisconnectingStrava(true)
    // Simulate API Call
    await new Promise((res) => setTimeout(res, 1000))
    setConnectionStatus((prev) => ({ ...prev, strava: "not_connected", stravaUsername: undefined }))
    toast({ title: "Strava Disconnected" })
    setIsDisconnectingStrava(false)
  }

  const handleSaveSpreadsheet = async () => {
    if (!inputSpreadsheetUrl.trim() || !inputSpreadsheetUrl.startsWith("https://docs.google.com/spreadsheets/d/")) {
      toast({
        title: "Invalid URL",
        description: "Please enter a valid Google Spreadsheet URL.",
        variant: "destructive",
      })
      return
    }
    setIsSavingUrl(true)
    await new Promise((res) => setTimeout(res, 1000)) // Simulate API
    setSpreadsheetUrl(inputSpreadsheetUrl)
    toast({ title: "Spreadsheet Link Saved!" })
    setIsSavingUrl(false)
  }

  const handleReauthorizeGoogle = async () => {
    setIsReauthorizingGoogle(true)
    // Simulate Google OAuth re-auth flow (likely redirect)
    await new Promise((res) => setTimeout(res, 1500))
    setConnectionStatus((prev) => ({ ...prev, google: "active" }))
    toast({ title: "Google Re-authorized" })
    setIsReauthorizingGoogle(false)
  }

  if (authLoading || isFetchingData) {
    return (
      <div className="flex items-center justify-center h-[calc(100vh-200px)]">
        <Loader2 className="h-10 w-10 animate-spin text-primary" />
      </div>
    )
  }

  const getStatusDisplay = (
    status: ConnectionStatus["google"] | ConnectionStatus["strava"],
    serviceName: string,
    username?: string,
  ) => {
    // Using default success/destructive colors from Tailwind config now
    // These are mapped to Academy Green (via --primary for success-like states) and a default red.
    let icon,
      text,
      badgeVariant: "default" | "secondary" | "destructive" | "outline" = "secondary",
      badgeText = ""
    switch (status) {
      case "active":
      case "connected":
        icon = <CheckCircle className="w-5 h-5 text-green-600" /> // Explicit green
        text = serviceName === "Strava" ? `Connected as ${username}` : "Sheets Access: Active"
        badgeVariant = "default" // This will use primary color by default in shadcn
        badgeText = "Active"
        break
      case "needs_reauth":
        icon = <AlertTriangle className="w-5 h-5 text-orange-500" /> // Standard warning orange
        text = `${serviceName} Access: Re-authorization Needed!`
        badgeVariant = "outline"
        badgeText = "Re-authorize"
        break
      case "not_connected":
        icon = <XCircle className="w-5 h-5 text-red-600" /> // Explicit red
        text = `${serviceName}: Not Connected`
        badgeVariant = "destructive"
        badgeText = "Not Connected"
        break
      default:
        icon = <AlertTriangle className="w-5 h-5 text-red-600" />
        text = `${serviceName} Access: Error`
        badgeVariant = "destructive"
        badgeText = "Error"
    }
    return { icon, text, badge: <Badge variant={badgeVariant}>{badgeText}</Badge> }
  }

  const googleStatus = getStatusDisplay(connectionStatus.google, "Google")
  const stravaStatus = getStatusDisplay(connectionStatus.strava, "Strava", connectionStatus.stravaUsername)

  return (
    <div className="space-y-6 max-w-3xl mx-auto font-body">
      {(connectionStatus.google === "needs_reauth" || connectionStatus.strava === "needs_reauth") && (
        <Card className="border-orange-500 bg-orange-50">
          <CardContent className="pt-6">
            <div className="flex items-center space-x-3">
              <AlertTriangle className="w-6 h-6 text-orange-600" />
              <p className="text-orange-700 font-semibold">Action Required: Some connections need re-authorization.</p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Google Account Card */}
      <Card className="bg-card shadow-md rounded-lg">
        <CardHeader>
          <CardTitle className="flex items-center text-2xl">
            <UserCircle2 className="mr-3 h-7 w-7" />
            Google Account
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-foreground/80">
            Connected as: <span className="font-semibold text-foreground">{user?.email}</span>
          </p>
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              {googleStatus.icon}
              <span className="text-sm text-foreground/90">{googleStatus.text}</span>
            </div>
            {googleStatus.badge}
          </div>
          {connectionStatus.google === "needs_reauth" && (
            <button
              className="btn-academy-secondary text-sm"
              onClick={handleReauthorizeGoogle}
              disabled={isReauthorizingGoogle}
            >
              {isReauthorizingGoogle ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
              Re-authorize Google
            </button>
          )}
        </CardContent>
      </Card>

      {/* Strava Connection Card */}
      <Card className="bg-card shadow-md rounded-lg">
        <CardHeader>
          <CardTitle className="flex items-center text-2xl">
            <Activity className="mr-3 h-7 w-7" />
            Strava Connection
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              {stravaStatus.icon}
              <span className="text-sm text-foreground/90">{stravaStatus.text}</span>
            </div>
            {stravaStatus.badge}
          </div>
          {connectionStatus.strava === "not_connected" && (
            <button className="btn-academy-primary" onClick={handleConnectStrava} disabled={isConnectingStrava}>
              {isConnectingStrava ? <Loader2 className="h-4 w-4 animate-spin" /> : <LinkIcon className="h-4 w-4" />}
              Connect to Strava
            </button>
          )}
          {connectionStatus.strava === "connected" && (
            <div className="flex space-x-3">
              <button
                className="btn-academy-secondary text-sm"
                onClick={handleReauthorizeStrava}
                disabled={isConnectingStrava}
              >
                {isConnectingStrava ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                Re-authorize
              </button>
              <button
                className="btn-academy-destructive"
                onClick={handleDisconnectStrava}
                disabled={isDisconnectingStrava}
              >
                {isDisconnectingStrava ? <Loader2 className="h-4 w-4 animate-spin" /> : <Unlink className="h-4 w-4" />}
                Disconnect
              </button>
            </div>
          )}
          {connectionStatus.strava === "needs_reauth" && (
            <button
              className="btn-academy-secondary text-sm"
              onClick={handleReauthorizeStrava}
              disabled={isConnectingStrava}
            >
              {isConnectingStrava ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
              Re-authorize Strava
            </button>
          )}
        </CardContent>
      </Card>

      {/* Spreadsheet Config Card */}
      <Card className="bg-card shadow-md rounded-lg">
        <CardHeader>
          <CardTitle className="flex items-center text-2xl">
            <FileSpreadsheet className="mr-3 h-7 w-7" />
            Spreadsheet Configuration
          </CardTitle>
          <CardDescription className="text-foreground/70 pt-1">
            Paste the full URL of your Google Spreadsheet for activity logging.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div>
            <Label htmlFor="spreadsheet-url" className="font-semibold text-foreground/90">
              Spreadsheet Link
            </Label>
            <div className="flex space-x-2 mt-1">
              <Input
                id="spreadsheet-url"
                type="url"
                placeholder="https://docs.google.com/spreadsheets/d/..."
                value={inputSpreadsheetUrl}
                onChange={(e) => setInputSpreadsheetUrl(e.target.value)}
                className="input-academy flex-grow" // Using custom class
              />
              <button
                className="btn-academy-primary"
                onClick={handleSaveSpreadsheet}
                disabled={isSavingUrl || inputSpreadsheetUrl === spreadsheetUrl}
              >
                {isSavingUrl ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                Save Link
              </button>
            </div>
          </div>
          {spreadsheetUrl && (
            <p className="text-sm text-foreground/80 flex items-center">
              Current:{" "}
              <a
                href={spreadsheetUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="ml-1 text-accent hover:underline truncate max-w-xs inline-block"
              >
                {spreadsheetUrl}
              </a>
              <ExternalLink className="ml-1 h-3 w-3 text-accent" />
            </p>
          )}
          {!spreadsheetUrl && <p className="text-sm text-foreground/80">Spreadsheet Link: Not Set.</p>}
        </CardContent>
      </Card>

      {/* Last Run Status Card */}
      <Card className="bg-card shadow-md rounded-lg">
        <CardHeader>
          <CardTitle className="flex items-center text-2xl">
            <Clock className="mr-3 h-7 w-7" />
            Last Automation Run
          </CardTitle>
        </CardHeader>
        <CardContent>
          {lastRunStatus ? (
            <div className="space-y-1">
              <div className="flex items-center justify-between">
                <p className="text-sm font-semibold text-foreground">
                  Last Sync:{" "}
                  {new Date(lastRunStatus.date).toLocaleDateString("en-US", {
                    year: "numeric",
                    month: "long",
                    day: "numeric",
                  })}
                </p>
                <Badge
                  variant={
                    lastRunStatus.status === "success"
                      ? "default"
                      : lastRunStatus.status === "partial_error"
                        ? "outline"
                        : "destructive"
                  }
                  className={
                    lastRunStatus.status === "success"
                      ? "bg-green-100 text-green-700 border-green-300"
                      : lastRunStatus.status === "partial_error"
                        ? "bg-orange-100 text-orange-700 border-orange-300"
                        : "bg-red-100 text-red-700 border-red-300"
                  }
                >
                  {lastRunStatus.status === "success"
                    ? "Success"
                    : lastRunStatus.status === "partial_error"
                      ? "Completed with Errors"
                      : "Failed"}
                </Badge>
              </div>
              <p className="text-sm text-foreground/80">{lastRunStatus.message}</p>
            </div>
          ) : (
            <p className="text-sm text-foreground/80">No automation run data available yet.</p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
