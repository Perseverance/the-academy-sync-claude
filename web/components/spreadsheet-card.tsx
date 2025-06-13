"use client"

import { useState } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { FileSpreadsheet, Edit3, Save, Info } from "lucide-react"
import type { SpreadsheetConfigStatus } from "@/context/app-state-provider"
import { useToast } from "@/hooks/use-toast"

interface SpreadsheetCardProps {
  status: SpreadsheetConfigStatus
  configuredUrl?: string
  onSave: (url: string) => void
  onChange: () => void
}

export function SpreadsheetCard({ status, configuredUrl, onSave, onChange }: SpreadsheetCardProps) {
  const [urlInput, setUrlInput] = useState(configuredUrl || "")
  const { toast } = useToast()

  const handleSave = () => {
    if (!urlInput.startsWith("https://docs.google.com/spreadsheets/d/")) {
      toast({
        title: "Invalid URL",
        description: "Please enter a valid Google Spreadsheet URL.",
        variant: "destructive",
      })
      return
    }
    onSave(urlInput)
  }

  const isDisabled = status === "Disabled"
  const isConfigured = status === "Configured"

  return (
    <Card className="shadow-md">
      <CardHeader>
        <CardTitle className="text-xl flex items-center gap-2">
          <FileSpreadsheet className="h-6 w-6" />
          Spreadsheet Configuration
        </CardTitle>
        {isConfigured && configuredUrl && (
          <CardDescription className="text-sm pt-1 block overflow-hidden">
            {" "}
            {/* Ensure parent can constrain */}
            Current Sheet:{" "}
            <a
              href={configuredUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="link-main truncate inline-block max-w-[calc(100%-theme(spacing.28))]" /* Adjust max-width as needed or use max-w-full if parent is well-constrained */
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
            <p className="text-sm">Please connect your Strava account first to enable spreadsheet configuration.</p>
          </div>
        )}

        {!isConfigured && !isDisabled && (
          <div>
            <Label htmlFor="spreadsheet-url" className="text-sm font-semibold text-foreground/90">
              Full Google Spreadsheet URL
            </Label>
            <div className="flex items-center space-x-2 mt-1">
              <Input
                id="spreadsheet-url"
                type="url"
                placeholder="https://docs.google.com/spreadsheets/d/..."
                value={urlInput}
                onChange={(e) => setUrlInput(e.target.value)}
                disabled={isDisabled}
                className="input-main"
              />
              <button onClick={handleSave} className="btn-primary-main px-4" disabled={isDisabled || !urlInput}>
                <Save className="h-4 w-4" />
                Save
              </button>
            </div>
          </div>
        )}

        {isConfigured && (
          <button onClick={onChange} className="btn-secondary-main w-full text-sm py-1.5 px-3">
            <Edit3 className="h-4 w-4" />
            Change Spreadsheet
          </button>
        )}
      </CardContent>
    </Card>
  )
}
