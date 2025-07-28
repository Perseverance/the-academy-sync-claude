"use client"

import { useTranslation } from "react-i18next"
import { usePageTitle } from "@/hooks/use-page-title"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { AcademyLogo } from "./icons/academy-logo"
import Link from "next/link"
import { ArrowLeft } from "lucide-react"

export function PrivacyPolicyPage() {
  const { t } = useTranslation()
  usePageTitle("privacy.pageTitle")

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
            <h1 className="text-2xl font-brand text-primary">{t('privacy.title')}</h1>
          </div>
        </div>
      </header>

      {/* Content */}
      <main className="container mx-auto px-4 sm:px-6 lg:px-8 py-8 max-w-4xl">
        <Card>
          <CardHeader>
            <CardTitle className="text-3xl font-brand">{t('privacy.heading')}</CardTitle>
            <p className="text-muted-foreground mt-2">{t('privacy.lastUpdated')}</p>
          </CardHeader>
          <CardContent className="prose prose-neutral dark:prose-invert max-w-none">
            {/* Introduction */}
            <section className="mb-8">
              <p className="text-foreground/80 leading-relaxed">
                {t('privacy.introduction')}
              </p>
            </section>

            {/* Data Collection */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('privacy.dataCollection.title')}</h2>
              <p className="mb-4">{t('privacy.dataCollection.description')}</p>
              <ul className="list-disc pl-6 space-y-2">
                <li>{t('privacy.dataCollection.items.account')}</li>
                <li>{t('privacy.dataCollection.items.strava')}</li>
                <li>{t('privacy.dataCollection.items.google')}</li>
                <li>{t('privacy.dataCollection.items.usage')}</li>
              </ul>
            </section>

            {/* Data Usage */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('privacy.dataUsage.title')}</h2>
              <p className="mb-4">{t('privacy.dataUsage.description')}</p>
              <ul className="list-disc pl-6 space-y-2">
                <li>{t('privacy.dataUsage.items.sync')}</li>
                <li>{t('privacy.dataUsage.items.automation')}</li>
                <li>{t('privacy.dataUsage.items.notifications')}</li>
                <li>{t('privacy.dataUsage.items.improvement')}</li>
              </ul>
            </section>

            {/* Data Security */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('privacy.dataSecurity.title')}</h2>
              <p>{t('privacy.dataSecurity.description')}</p>
            </section>

            {/* Third Party Services */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('privacy.thirdParty.title')}</h2>
              <p className="mb-4">{t('privacy.thirdParty.description')}</p>
              <ul className="list-disc pl-6 space-y-2">
                <li>
                  <strong>Google:</strong> {t('privacy.thirdParty.google')}
                </li>
                <li>
                  <strong>Strava:</strong> {t('privacy.thirdParty.strava')}
                </li>
              </ul>
            </section>

            {/* User Rights */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('privacy.userRights.title')}</h2>
              <p className="mb-4">{t('privacy.userRights.description')}</p>
              <ul className="list-disc pl-6 space-y-2">
                <li>{t('privacy.userRights.items.access')}</li>
                <li>{t('privacy.userRights.items.deletion')}</li>
                <li>{t('privacy.userRights.items.export')}</li>
                <li>{t('privacy.userRights.items.correction')}</li>
              </ul>
            </section>

            {/* Data Retention */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('privacy.dataRetention.title')}</h2>
              <p>{t('privacy.dataRetention.description')}</p>
            </section>

            {/* Contact */}
            <section className="mb-8">
              <h2 className="text-2xl font-semibold mb-4">{t('privacy.contact.title')}</h2>
              <p>
                {t('privacy.contact.description')}{' '}
                <a href="mailto:privacy@theacademy.com" className="text-primary hover:underline">
                  privacy@theacademy.com
                </a>
              </p>
            </section>

            {/* Changes */}
            <section>
              <h2 className="text-2xl font-semibold mb-4">{t('privacy.changes.title')}</h2>
              <p>{t('privacy.changes.description')}</p>
            </section>
          </CardContent>
        </Card>
      </main>
    </div>
  )
}