import type React from "react"
import type { Metadata } from "next"
import { Inter, Exo_2 as Exo2 } from "next/font/google" // Updated font import
import "./globals.css"
import { AppStateProvider } from "@/context/app-state-provider" // Renamed for clarity
import { Toaster } from "@/components/ui/toaster"
import { ThemeProvider } from "@/components/theme-provider"
import { I18nProvider } from "@/src/i18n-provider"
import { Footer } from "@/components/footer"

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

// Dynamic metadata will be set by pages
export const metadata: Metadata = {
  // Title is set dynamically by usePageTitle hook
  description: "Automated training log synchronization for athletes",
  generator: "v0.dev",
  icons: {
    icon: '/favicon.svg',
    shortcut: '/favicon.svg',
    apple: '/favicon.svg',
    other: {
      rel: 'icon',
      url: '/favicon.svg',
      type: 'image/svg+xml',
    },
  },
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={`${inter.variable} ${exo2.variable} font-body antialiased bg-background text-foreground`}>
        <ThemeProvider attribute="class" defaultTheme="light" enableSystem={false} disableTransitionOnChange>
          <I18nProvider>
            <AppStateProvider>
              <div className="flex flex-col min-h-screen">
                {children}
                <Footer />
              </div>
              <Toaster />
            </AppStateProvider>
          </I18nProvider>
        </ThemeProvider>
      </body>
    </html>
  )
}
