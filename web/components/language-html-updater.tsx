"use client"

import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'

export function LanguageHtmlUpdater() {
  const { i18n } = useTranslation()
  
  useEffect(() => {
    // Update the html lang attribute when language changes
    document.documentElement.lang = i18n.language
  }, [i18n.language])
  
  return null
}