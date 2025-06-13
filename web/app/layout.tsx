import type React from "react"
import type { Metadata } from "next"
import { Inter, Exo_2 as Exo2 } from "next/font/google" // Updated font import
import "./globals.css"
import { AppStateProvider } from "@/context/app-state-provider" // Renamed for clarity
import { Toaster } from "@/components/ui/toaster"
import { ThemeProvider } from "@/components/theme-provider"

const inter = Inter({
  subsets: ["latin", "cyrillic"],
  weight: ["400", "600", "700"],
  variable: "--font-inter",
})

const exo2 = Exo2({
  // Updated font name
  subsets: ["latin", "cyrillic"],
  weight: ["700"],
  variable: "--font-exo2",
})

export const metadata: Metadata = {
  title: "Strava Log Automator - Configuration",
  description: "Configure your automated Strava running log to Google Sheets.",
  generator: "v0.dev",
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={`${inter.variable} ${exo2.variable} font-body antialiased bg-background text-foreground`}>
        <ThemeProvider attribute="class" defaultTheme="light" enableSystem={false} disableTransitionOnChange>
          <AppStateProvider>
            {children}
            <Toaster />
          </AppStateProvider>
        </ThemeProvider>
      </body>
    </html>
  )
}
