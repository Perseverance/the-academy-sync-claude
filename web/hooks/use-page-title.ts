import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'

export function usePageTitle(titleKey: string) {
  const { t, i18n, ready } = useTranslation()
  
  useEffect(() => {
    // Update title immediately and whenever dependencies change
    const updateTitle = () => {
      const title = t(titleKey)
      // Always update the title
      document.title = title
    }
    
    // Update on mount and when ready status changes
    updateTitle()
    
    // Also listen for language changes
    const handleLanguageChange = () => {
      updateTitle()
    }
    
    i18n.on('languageChanged', handleLanguageChange)
    
    return () => {
      i18n.off('languageChanged', handleLanguageChange)
    }
  }, [t, i18n, titleKey, ready, i18n.language]) // Added i18n.language to dependencies
}