"use client"

import * as React from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { AlertTriangle, CheckCircle, LinkIcon, LogOut, RefreshCw } from "lucide-react"
import type { ServiceStatus } from "@/context/app-state-provider"
import { getCachedAvatarUrl } from "@/lib/avatar-utils"
import { useTranslation } from "react-i18next"

interface ConnectionCardProps {
  serviceName: string
  serviceIcon: React.ReactNode
  status: ServiceStatus
  userName?: string
  userAvatarUrl?: string
  onConnect: () => void
  onDisconnect: () => void
  onReauthorize: () => void
  isGoogle?: boolean // Special handling for Google if needed
  isStrava?: boolean // Special handling for Strava if needed
}

export function ConnectionCard({
  serviceName,
  serviceIcon,
  status,
  userName,
  userAvatarUrl,
  onConnect,
  onDisconnect,
  onReauthorize,
  isGoogle = false,
  isStrava = false,
}: ConnectionCardProps) {
  const { t } = useTranslation()
  
  const renderContent = () => {
    switch (status) {
      case "NotConnected":
        return (
          <>
            <div className="flex items-center space-x-3 mb-3">
              {serviceIcon}
              <Badge className="bg-orange-100 text-orange-800 border-orange-200">
                {t('connectionCard.status.notConnected')}
              </Badge>
            </div>
            {isStrava ? (
              <button 
                onClick={onConnect} 
                className="flex justify-start items-center hover:opacity-90 transition-opacity focus:outline-none focus:ring-2 focus:ring-orange-500 focus:ring-offset-2 rounded-md"
                aria-label={t('connectionCard.actions.connectStrava')}
              >
                <img 
                  src="/icons/btn_strava_connect_with_orange.svg" 
                  alt={t('connectionCard.actions.connectStrava')}
                  className="h-9 w-auto"
                />
              </button>
            ) : (
              <button onClick={onConnect} className="btn-primary-main w-full">
                <LinkIcon className="h-4 w-4" />
                {t('connectionCard.actions.connect', { serviceName })}
              </button>
            )}
          </>
        )
      case "Connected":
        return (
          <>
            <div className="flex items-center space-x-3 mb-3">
              <Avatar className="h-10 w-10">
                <AvatarImage src={getCachedAvatarUrl(userAvatarUrl)} alt={userName} />
                <AvatarFallback>{userName ? userName.charAt(0).toUpperCase() : "?"}</AvatarFallback>
              </Avatar>
              <div>
                <p className="text-sm font-semibold text-foreground">{userName || serviceName}</p>
                <Badge className="bg-success/20 text-success border-success/30">
                  <CheckCircle className="h-3 w-3 mr-1" /> {t('connectionCard.status.active')}
                </Badge>
              </div>
            </div>
            {!isGoogle && ( // Typically don't disconnect Google session from here, sign out instead
              <button onClick={onDisconnect} className="btn-destructive-main w-full text-sm py-1.5 px-3">
                <LogOut className="h-4 w-4" />
                {t('connectionCard.actions.disconnect')}
              </button>
            )}
          </>
        )
      case "ReauthorizationNeeded":
        return (
          <>
            <div className="flex items-center space-x-3 mb-3 text-warning-foreground bg-warning/10 p-3 rounded-md border border-warning/30">
              <AlertTriangle className="h-6 w-6 text-warning" />
              <p className="text-sm font-semibold">{t('connectionCard.status.reauthRequired')}</p>
            </div>
            <button onClick={onReauthorize} className="btn-primary-main w-full">
              <RefreshCw className="h-4 w-4" />
              {t('connectionCard.actions.reauthorize', { serviceName })}
            </button>
          </>
        )
      default:
        return <p>{t('connectionCard.error.unknownStatus')}</p>
    }
  }

  return (
    <Card className="shadow-md">
      <CardHeader>
        <CardTitle className="text-xl flex items-center gap-2">
          {serviceIcon}
          {serviceName}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">{renderContent()}</CardContent>
    </Card>
  )
}
