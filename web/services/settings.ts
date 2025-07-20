export interface UpdateSettingsRequest {
  automation_enabled: boolean
  email_notifications_enabled: boolean
}

class SettingsService {
  private readonly baseURL: string

  constructor() {
    // Use environment variable for API URL, fallback to localhost
    this.baseURL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  }

  async updateSettings(settings: UpdateSettingsRequest) {
    const response = await fetch(`${this.baseURL}/api/settings`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include', // Important for cookie authentication
      body: JSON.stringify(settings),
    })

    if (!response.ok) {
      const errorText = await response.text()
      throw new Error(errorText || 'Failed to update settings')
    }

    return response.json()
  }
}

export const settingsService = new SettingsService()