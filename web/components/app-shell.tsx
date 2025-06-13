"use client"

import type React from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { Button as ShadButton } from "@/components/ui/button" // Keep shadcn button for dropdown trigger
import { useAuth } from "@/components/auth-provider"
import { cn } from "@/lib/utils"
import { LayoutDashboard, ListChecks, LogOut, Settings, Loader2 } from "lucide-react"
// Removed Moon, Sun as theme toggle might be out of scope for this light theme
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { AcademyLogo } from "@/components/icons/academy-logo"

const navItems = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/logs", label: "Log Summaries", icon: ListChecks },
]

export function AppShell({ children }: { children: React.ReactNode }) {
  const { user, signOut, isLoading } = useAuth()
  const pathname = usePathname()
  // const { theme, setTheme } = useTheme() // Theme toggle removed for now

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <Loader2 className="h-12 w-12 animate-spin text-primary" />
      </div>
    )
  }

  if (!user) {
    return null
  }

  return (
    <div className="min-h-screen flex bg-background">
      {/* Sidebar */}
      <aside className="w-64 bg-card border-r border-border flex flex-col p-4 space-y-6 sticky top-0 h-screen shadow-md">
        <Link href="/dashboard" className="flex items-center space-x-2 px-2 py-3">
          <AcademyLogo className="h-10 w-10" />
          <h1 className="text-2xl font-brand font-bold text-primary">Sync Tool</h1>
        </Link>
        <nav className="flex-grow">
          <ul className="space-y-2">
            {navItems.map((item) => (
              <li key={item.href}>
                <Link
                  href={item.href}
                  className={cn(
                    "flex items-center w-full justify-start text-base py-3 px-4 rounded-lg transition-colors duration-150 font-medium",
                    pathname === item.href
                      ? "bg-primary/10 text-primary" // Academy Green with slight background
                      : "text-foreground hover:bg-muted hover:text-primary",
                  )}
                >
                  <item.icon className="mr-3 h-5 w-5" />
                  {item.label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
        {/* Theme toggle removed, can be added back if needed
        <div className="mt-auto">
          <button
            className="flex items-center w-full justify-start text-base py-3 px-4 rounded-lg text-foreground hover:bg-muted hover:text-primary font-medium"
            onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
          >
            {theme === "dark" ? <Sun className="mr-3 h-5 w-5" /> : <Moon className="mr-3 h-5 w-5" />}
            Toggle Theme
          </button>
        </div>
        */}
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col">
        {/* Top Bar */}
        <header className="bg-card border-b border-border p-4 sticky top-0 z-10 shadow-sm">
          <div className="container mx-auto flex justify-end items-center max-w-full px-4 sm:px-6 lg:px-8">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <ShadButton variant="ghost" className="flex items-center space-x-2 p-2 rounded-full hover:bg-muted">
                  <img
                    src={user.picture || "/placeholder.svg?height=32&width=32&query=avatar"}
                    alt="User avatar"
                    className="w-8 h-8 rounded-full border-2 border-primary"
                  />
                  <span className="text-sm font-medium text-foreground hidden md:inline">{user.name}</span>
                </ShadButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-56 bg-card border-border shadow-lg rounded-md">
                <DropdownMenuLabel className="font-normal px-3 py-2">
                  <div className="flex flex-col space-y-1">
                    <p className="text-sm font-semibold leading-none text-foreground">{user.name}</p>
                    <p className="text-xs leading-none text-muted-foreground">{user.email}</p>
                  </div>
                </DropdownMenuLabel>
                <DropdownMenuSeparator className="bg-border" />
                <DropdownMenuItem
                  className="hover:bg-muted cursor-pointer px-3 py-2"
                  onClick={() => {
                    // router.push('/settings') // Example
                  }}
                >
                  <Settings className="mr-2 h-4 w-4 text-muted-foreground" />
                  <span className="text-foreground">Settings</span>
                </DropdownMenuItem>
                <DropdownMenuSeparator className="bg-border" />
                <DropdownMenuItem className="hover:bg-muted cursor-pointer px-3 py-2" onClick={signOut}>
                  <LogOut className="mr-2 h-4 w-4 text-muted-foreground" />
                  <span className="text-foreground">Sign Out</span>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </header>

        {/* Page Content */}
        <main className="flex-1 p-6 sm:p-8 lg:p-10 overflow-y-auto">{children}</main>
      </div>
    </div>
  )
}
