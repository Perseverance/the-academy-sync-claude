"use client"

import { useAppState } from "@/context/app-state-provider"
import { AcademyLogo } from "@/components/icons/academy-logo"
import { GoogleLogo } from "@/components/icons/google-logo"
import { HowItWorks } from "@/components/how-it-works"
import { SpreadsheetExample } from "@/components/spreadsheet-example"
import { Loader2 } from "lucide-react"
import { useTranslation } from "react-i18next"
import { usePageTitle } from "@/hooks/use-page-title"

export function SignInPage() {
  const { state, actions } = useAppState()
  const { t } = useTranslation()
  usePageTitle('metadata.title')

  return (
    <div className="min-h-screen bg-background">
      {/* Hero Section */}
      <div className="flex flex-col items-center justify-center p-6 pt-12 text-center">
        <AcademyLogo className="w-20 h-20 mb-6" />
        <h1 className="text-4xl md:text-5xl font-brand font-bold text-primary mb-4">{t('signIn.title')}</h1>
        <p className="text-lg text-muted-foreground max-w-xl mb-8">
          {t('signIn.description')}
        </p>
        
        {/* Sign In Button */}
        <div className="mb-12">
          <button onClick={actions.signIn} className="btn-primary-main text-lg px-8 py-3" disabled={state.isAuthLoading}>
            {state.isAuthLoading ? <Loader2 className="h-5 w-5 animate-spin" /> : <GoogleLogo className="h-5 w-5" />}
            {t('signIn.signInButton')}
          </button>
        </div>
        
        {/* How It Works Section */}
        <HowItWorks />
        
        {/* Spreadsheet Example */}
        <SpreadsheetExample />
      </div>
    </div>
  )
}
