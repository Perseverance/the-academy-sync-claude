"use client"

import { useAppState } from "@/context/app-state-provider"
import { AcademyLogo } from "@/components/icons/academy-logo"
import { GoogleLogo } from "@/components/icons/google-logo"
import { Loader2 } from "lucide-react"

export function SignInPage() {
  const { state, actions } = useAppState()

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-background p-6 text-center">
      <AcademyLogo className="w-20 h-20 mb-6" />
      <h1 className="text-4xl md:text-5xl font-brand font-bold text-primary mb-4">Automate Your Strava Running Log</h1>
      <p className="text-lg text-muted-foreground max-w-xl mb-8">
        Connect your Strava and Google accounts to automatically sync your running activities to a Google Spreadsheet.
        Focus on your training, not manual data entry.
      </p>
      <button onClick={actions.signIn} className="btn-primary-main text-lg px-8 py-3" disabled={state.isAuthLoading}>
        {state.isAuthLoading ? <Loader2 className="h-5 w-5 animate-spin" /> : <GoogleLogo className="h-5 w-5" />}
        Sign in with Google
      </button>
    </div>
  )
}
