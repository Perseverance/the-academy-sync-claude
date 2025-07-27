"use client"

import { useEffect } from 'react'
import i18n from './i18n'
import { LanguageHtmlUpdater } from '@/components/language-html-updater'

export function I18nProvider({ children }: { children: React.ReactNode }) {
  useEffect(() => {
    // i18n is already initialized in the i18n.js file
    // This component just ensures it happens on the client side
  }, [])

  return (
    <>
      <LanguageHtmlUpdater />
      {children}
    </>
  )
}