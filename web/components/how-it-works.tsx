"use client"

import React from 'react'
import { useTranslation } from 'react-i18next'
import { Activity, Link, Zap, Shield, ArrowRight } from 'lucide-react'
import Image from 'next/image'
import { AcademyLogo } from '@/components/icons/academy-logo'

export function HowItWorks() {
  const { t } = useTranslation()

  const steps = [
    {
      icon: Activity,
      title: t('signIn.howItWorks.step1.title'),
      description: t('signIn.howItWorks.step1.description'),
      screenshot: '/images/strava-activities-screenshot.png',
      screenshotAlt: t('signIn.screenshots.stravaAlt')
    },
    {
      icon: Link,
      title: t('signIn.howItWorks.step2.title'),
      description: t('signIn.howItWorks.step2.description'),
      customVisual: 'tas-logo'
    },
    {
      icon: Zap,
      title: t('signIn.howItWorks.step3.title'),
      description: t('signIn.howItWorks.step3.description'),
      screenshot: '/images/google-sheets-screenshot.png',
      screenshotAlt: t('signIn.screenshots.sheetsAlt')
    },
  ]

  return (
    <div className="w-full max-w-6xl mx-auto py-8">
      <h2 className="text-2xl font-bold text-center mb-8">{t('signIn.howItWorks.title')}</h2>
      
      <div className="grid lg:grid-cols-3 gap-8 items-start">
        {steps.map((step, index) => (
          <div key={index} className="flex flex-col items-center text-center">
            <div className="w-16 h-16 bg-primary/10 rounded-full flex items-center justify-center mb-4">
              <step.icon className="w-8 h-8 text-primary" />
            </div>
            <h3 className="font-semibold mb-2">{step.title}</h3>
            <p className="text-sm text-muted-foreground mb-4">{step.description}</p>
            
            {step.screenshot && (
              <div className="relative w-full">
                <div className="rounded-lg border border-border overflow-hidden shadow-md">
                  <Image
                    src={step.screenshot}
                    alt={step.screenshotAlt}
                    width={320}
                    height={240}
                    className="w-full h-auto object-cover"
                  />
                </div>
                {index === 0 && (
                  <div className="absolute -right-4 top-8 hidden lg:block">
                    <ArrowRight className="w-6 h-6 text-muted-foreground" />
                  </div>
                )}
              </div>
            )}
            
            {step.customVisual === 'tas-logo' && (
              <div className="relative w-full">
                <div className="rounded-lg border border-border bg-muted/10 p-8 shadow-md">
                  <AcademyLogo className="w-32 h-32 mx-auto" />
                  <p className="text-xs text-muted-foreground text-center mt-4">The Academy Sync</p>
                </div>
                {index === 1 && (
                  <div className="absolute -right-4 top-8 hidden lg:block">
                    <ArrowRight className="w-6 h-6 text-muted-foreground" />
                  </div>
                )}
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Data Usage Section */}
      <div className="mt-12 bg-muted/30 rounded-lg p-6">
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-5 h-5 text-primary" />
          <h3 className="font-semibold">{t('signIn.dataUsage.title')}</h3>
        </div>
        
        <div className="space-y-3 text-sm">
          <div className="flex items-start gap-2">
            <div className="w-1.5 h-1.5 bg-primary rounded-full mt-1.5 flex-shrink-0" />
            <p>{t('signIn.dataUsage.strava')}</p>
          </div>
          <div className="flex items-start gap-2">
            <div className="w-1.5 h-1.5 bg-primary rounded-full mt-1.5 flex-shrink-0" />
            <p>{t('signIn.dataUsage.sheets')}</p>
          </div>
          <div className="flex items-start gap-2">
            <div className="w-1.5 h-1.5 bg-primary rounded-full mt-1.5 flex-shrink-0" />
            <p>{t('signIn.dataUsage.privacy')}</p>
          </div>
        </div>
      </div>
    </div>
  )
}