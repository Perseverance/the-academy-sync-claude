// Strava connection service for handling OAuth flow and connection management
export interface StravaAuthResponse {
  auth_url: string
}

export interface StravaError {
  error: string
  message: string
}

class StravaService {
  private readonly baseURL: string

  constructor() {
    // Use environment variable for API URL, fallback to localhost
    this.baseURL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  }

  /**
   * Get the Strava OAuth authorization URL and redirect to it
   */
  async initiateStravaConnection(): Promise<void> {
    try {
      const response = await fetch(`${this.baseURL}/api/connections/strava`, {
        method: 'GET',
        credentials: 'include', // Include cookies for session management
      })

      if (!response.ok) {
        throw new Error(`Failed to get Strava auth URL: ${response.status}`)
      }

      const data: StravaAuthResponse = await response.json()
      
      // Redirect to Strava OAuth
      window.location.href = data.auth_url
    } catch (error) {
      console.error('Error initiating Strava connection:', error)
      throw new Error('Failed to initiate Strava connection')
    }
  }

  /**
   * Disconnect the user's Strava account
   */
  async disconnectStrava(): Promise<void> {
    try {
      const response = await fetch(`${this.baseURL}/api/connections/strava`, {
        method: 'DELETE',
        credentials: 'include', // Include cookies for session
      })

      if (!response.ok) {
        throw new Error(`Failed to disconnect Strava: ${response.status}`)
      }

      // Response should be JSON with success message
      await response.json()
    } catch (error) {
      console.error('Error disconnecting Strava:', error)
      throw new Error('Failed to disconnect Strava account')
    }
  }
}

// Create singleton instance
export const stravaService = new StravaService()

// Export default instance
export default stravaService