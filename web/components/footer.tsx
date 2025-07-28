"use client"

import Link from "next/link"
import { useTranslation } from "react-i18next"

export function Footer() {
  const { t } = useTranslation()
  const currentYear = new Date().getFullYear()

  return (
    <footer className="border-t border-border bg-card mt-auto">
      <div className="container mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
          {/* Copyright */}
          <div className="text-sm text-muted-foreground">
            {t('footer.copyright', { year: currentYear })}
          </div>

          {/* Links */}
          <nav className="flex items-center gap-6">
            <Link 
              href="/privacy/" 
              className="text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              {t('footer.privacy')}
            </Link>
            <Link 
              href="/terms/" 
              className="text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              {t('footer.terms')}
            </Link>
          </nav>
        </div>
      </div>
    </footer>
  )
}