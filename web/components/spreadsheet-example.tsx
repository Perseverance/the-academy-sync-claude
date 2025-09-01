"use client"

import React from 'react'
import { useTranslation } from 'react-i18next'
import { Calendar, Activity, Timer, Route, FileText, TrendingUp } from 'lucide-react'

export function SpreadsheetExample() {
  const { t } = useTranslation()
  
  const fields = [
    {
      icon: Calendar,
      label: t('signIn.example.date'),
      example: '18.7.2025'
    },
    {
      icon: Activity,
      label: t('signIn.example.type'),
      example: t('signIn.example.typeExample')
    },
    {
      icon: Route,
      label: t('signIn.example.distance'),
      example: '10 km'
    },
    {
      icon: Timer,
      label: t('signIn.example.time'),
      example: '45:30'
    },
    {
      icon: TrendingUp,
      label: 'RPE',
      example: '5/10'
    },
    {
      icon: FileText,
      label: t('signIn.example.description'),
      example: t('signIn.example.descriptionExample')
    }
  ]

  return (
    <div className="w-full max-w-4xl mx-auto py-8">
      <h3 className="text-lg font-semibold mb-6 text-center">{t('signIn.example.title')}</h3>
      
      <div className="bg-muted/20 rounded-lg p-6">
        <div className="grid md:grid-cols-2 gap-4">
          {fields.map((field, index) => (
            <div key={index} className="flex items-start gap-3">
              <div className="w-10 h-10 bg-primary/10 rounded-lg flex items-center justify-center flex-shrink-0">
                <field.icon className="w-5 h-5 text-primary" />
              </div>
              <div className="flex-1">
                <p className="font-medium text-sm">{field.label}</p>
                <p className="text-sm text-muted-foreground">{field.example}</p>
              </div>
            </div>
          ))}
        </div>
        
        <p className="text-xs text-muted-foreground text-center mt-6">
          {t('signIn.example.note')}
        </p>
      </div>
    </div>
  )
}