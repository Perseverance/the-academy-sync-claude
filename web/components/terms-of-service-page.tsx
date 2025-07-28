"use client"

import { useTranslation } from "react-i18next"
import { usePageTitle } from "@/hooks/use-page-title"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { AcademyLogo } from "./icons/academy-logo"
import Link from "next/link"
import { ArrowLeft } from "lucide-react"

export function TermsOfServicePage() {
  const { t } = useTranslation()
  usePageTitle("terms.pageTitle")

  return (
    <div className="flex-1 bg-background">
      {/* Header */}
      <header className="bg-card border-b border-border shadow-sm sticky top-0 z-30">
        <div className="container mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Link href="/" className="flex items-center gap-3 text-muted-foreground hover:text-foreground transition-colors">
              <ArrowLeft className="h-5 w-5" />
              <AcademyLogo className="h-8 w-8" />
            </Link>
            <h1 className="text-2xl font-brand text-primary">{t('terms.title')}</h1>
          </div>
        </div>
      </header>

      {/* Content */}
      <main className="container mx-auto px-4 sm:px-6 lg:px-8 py-8 max-w-4xl">
        <Card>
          <CardHeader>
            <CardTitle className="text-3xl font-brand">{t('terms.heading')}</CardTitle>
            <p className="text-muted-foreground mt-2">{t('terms.lastUpdated')}</p>
          </CardHeader>
          <CardContent className="prose prose-neutral dark:prose-invert max-w-none">
            {/* Introduction */}
            <section className="mb-8">
              <p className="text-foreground/80 leading-relaxed">
                {t('terms.introduction')}
              </p>
            </section>

            {/* Acceptance */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('terms.acceptance.title')}</h2>
              <p>{t('terms.acceptance.description')}</p>
            </section>

            {/* Service Description */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('terms.serviceDescription.title')}</h2>
              <p className="mb-4">{t('terms.serviceDescription.description')}</p>
              <ul className="list-disc pl-6 space-y-2">
                <li>{t('terms.serviceDescription.items.sync')}</li>
                <li>{t('terms.serviceDescription.items.automation')}</li>
                <li>{t('terms.serviceDescription.items.notifications')}</li>
              </ul>
            </section>

            {/* User Accounts */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('terms.userAccounts.title')}</h2>
              <p className="mb-4">{t('terms.userAccounts.description')}</p>
              <ul className="list-disc pl-6 space-y-2">
                <li>{t('terms.userAccounts.items.accuracy')}</li>
                <li>{t('terms.userAccounts.items.security')}</li>
                <li>{t('terms.userAccounts.items.responsibility')}</li>
                <li>{t('terms.userAccounts.items.authorization')}</li>
              </ul>
            </section>

            {/* User Responsibilities */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('terms.userResponsibilities.title')}</h2>
              <p className="mb-4">{t('terms.userResponsibilities.description')}</p>
              <ul className="list-disc pl-6 space-y-2">
                <li>{t('terms.userResponsibilities.items.lawful')}</li>
                <li>{t('terms.userResponsibilities.items.accurate')}</li>
                <li>{t('terms.userResponsibilities.items.respectful')}</li>
                <li>{t('terms.userResponsibilities.items.security')}</li>
              </ul>
            </section>

            {/* Third Party Services */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('terms.thirdPartyServices.title')}</h2>
              <p className="mb-4">{t('terms.thirdPartyServices.description')}</p>
              <ul className="list-disc pl-6 space-y-2">
                <li>{t('terms.thirdPartyServices.items.google')}</li>
                <li>{t('terms.thirdPartyServices.items.strava')}</li>
                <li>{t('terms.thirdPartyServices.items.compliance')}</li>
              </ul>
            </section>

            {/* Intellectual Property */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('terms.intellectualProperty.title')}</h2>
              <p>{t('terms.intellectualProperty.description')}</p>
            </section>

            {/* Disclaimers */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('terms.disclaimers.title')}</h2>
              <p className="mb-4">{t('terms.disclaimers.description')}</p>
              <ul className="list-disc pl-6 space-y-2">
                <li>{t('terms.disclaimers.items.asIs')}</li>
                <li>{t('terms.disclaimers.items.availability')}</li>
                <li>{t('terms.disclaimers.items.accuracy')}</li>
                <li>{t('terms.disclaimers.items.fitness')}</li>
              </ul>
            </section>

            {/* Limitation of Liability */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('terms.limitation.title')}</h2>
              <p>{t('terms.limitation.description')}</p>
            </section>

            {/* Termination */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('terms.termination.title')}</h2>
              <p>{t('terms.termination.description')}</p>
            </section>

            {/* Governing Law */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('terms.governingLaw.title')}</h2>
              <p>{t('terms.governingLaw.description')}</p>
            </section>

            {/* Changes */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('terms.changes.title')}</h2>
              <p>{t('terms.changes.description')}</p>
            </section>

            {/* Contact */}
            <section>
              <h2 className="text-2xl font-semibold mb-4">{t('terms.contact.title')}</h2>
              <p>
                {t('terms.contact.description')}{' '}
                <a href="mailto:legal@theacademy.com" className="text-primary hover:underline">
                  legal@theacademy.com
                </a>
              </p>
            </section>
          </CardContent>
        </Card>
      </main>
    </div>
  )
}