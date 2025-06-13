"use client"

import { LogSummaries } from "@/components/log-summaries"
import { AppShell } from "@/components/app-shell"

export const dynamic = 'force-dynamic'

export default function LogsPage() {
  return (
    <AppShell>
      <LogSummaries />
    </AppShell>
  )
}
