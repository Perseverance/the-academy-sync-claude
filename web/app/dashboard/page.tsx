"use client"

import { DashboardPage } from "@/components/dashboard-page"
import { usePageTitle } from "@/hooks/use-page-title"

export const dynamic = 'force-dynamic'

export default function DashboardRoute() {
  usePageTitle('metadata.title')
  return <DashboardPage />
}
