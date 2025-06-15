// Authentication service for handling Google OAuth and session management
export interface ActivityLog {
  id: string
  date: string
  status: 'Success' | 'Failure' | 'SuccessWithWarning'
  summary: string
}

export interface User {
  id: number
  email: string
  name: string
  profile_picture_url?: string
  timezone: string
  email_notifications_enabled: boolean
  automation_enabled: boolean
  has_strava_connection: boolean
  has_sheets_connection: boolean
  strava_athlete_name?: string
  strava_profile_picture_url?: string
  recent_activity_logs: ActivityLog[]
}

export interface AuthResponse {
  auth_url: string
}

export interface AuthError {
  error: string
  message: string
}

class AuthService {
  private readonly baseURL: string

  constructor() {
    // Use environment variable for API URL, fallback to localhost
    this.baseURL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  }

  /**
   * Get the Google OAuth authorization URL
   */
  async getGoogleAuthURL(): Promise<string> {
    try {
      const response = await fetch(`${this.baseURL}/api/auth/google`, {
        method: 'GET',
        credentials: 'include', // Include cookies for session management
      })

      if (!response.ok) {
        throw new Error(`Failed to get auth URL: ${response.status}`)
      }

      const data: AuthResponse = await response.json()
      return data.auth_url
    } catch (error) {
      console.error('Error getting Google auth URL:', error)
      throw new Error('Failed to initiate Google authentication')
    }
  }

  /**
   * Get current authenticated user information
   */
  async getCurrentUser(): Promise<User | null> {
    try {
      const response = await fetch(`${this.baseURL}/api/auth/me`, {
        method: 'GET',
        credentials: 'include', // Include cookies for session
      })

      if (response.status === 401) {
        // User not authenticated
        return null
      }

      if (!response.ok) {
        throw new Error(`Failed to get user info: ${response.status}`)
      }

      const user: User = await response.json()
      return user
    } catch (error) {
      console.error('Error getting current user:', error)
      return null
    }
  }

  /**
   * Check if user is currently authenticated
   */
  async checkAuthStatus(): Promise<{ isAuthenticated: boolean; user: User | null }> {
    const user = await this.getCurrentUser()
    return {
      isAuthenticated: user !== null,
      user
    }
  }

  /**
   * Refresh the authentication token
   */
  async refreshToken(): Promise<boolean> {
    try {
      const response = await fetch(`${this.baseURL}/api/auth/refresh`, {
        method: 'POST',
        credentials: 'include',
      })

      return response.ok
    } catch (error) {
      console.error('Error refreshing token:', error)
      return false
    }
  }

  /**
   * Sign out the current user
   */
  async signOut(): Promise<void> {
    try {
      await fetch(`${this.baseURL}/api/auth/logout`, {
        method: 'POST',
        credentials: 'include',
      })
    } catch (error) {
      console.error('Error during sign out:', error)
      // Don't throw - we want to clear local state regardless
    }
  }

  /**
   * Initiate Google OAuth flow
   * This redirects the user to Google's consent screen
   */
  async initiateGoogleOAuth(): Promise<void> {
    const authURL = await this.getGoogleAuthURL()
    
    // Redirect to Google OAuth
    window.location.href = authURL
  }
}

// Create singleton instance
export const authService = new AuthService()

// Export default instance
export default authService