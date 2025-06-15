// Configuration service for handling spreadsheet configuration and other user settings

export interface SetSpreadsheetRequest {
  url: string
}

export interface SetSpreadsheetResponse {
  success: boolean
  message: string
}

export interface ConfigError {
  error: string
  message: string
  type?: string
}

export class ConfigService {
  private readonly baseURL: string

  constructor() {
    // Use environment variable for API URL, fallback to localhost
    this.baseURL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  }

  /**
   * Set the user's Google Spreadsheet URL
   */
  async setSpreadsheetUrl(url: string): Promise<void> {
    try {
      const response = await fetch(`${this.baseURL}/api/config/spreadsheet`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include', // Include cookies for authentication
        body: JSON.stringify({ url }),
      })

      if (!response.ok) {
        // Try to parse error response
        try {
          const errorData: ConfigError = await response.json()
          throw new ConfigApiError(
            errorData.error || 'UNKNOWN_ERROR',
            errorData.message || 'Failed to set spreadsheet configuration',
            errorData.type,
            response.status
          )
        } catch (parseError) {
          // If we can't parse the error response, throw a generic error
          throw new ConfigApiError(
            'HTTP_ERROR',
            `HTTP ${response.status}: ${response.statusText}`,
            undefined,
            response.status
          )
        }
      }

      const result: SetSpreadsheetResponse = await response.json()
      if (!result.success) {
        throw new ConfigApiError(
          'API_ERROR',
          result.message || 'Failed to set spreadsheet configuration',
          undefined,
          response.status
        )
      }
    } catch (error) {
      if (error instanceof ConfigApiError) {
        throw error
      }

      // Network or other unexpected errors
      console.error('Error setting spreadsheet URL:', error)
      throw new ConfigApiError(
        'NETWORK_ERROR',
        'Unable to connect to the server. Please check your internet connection and try again.',
        undefined,
        0
      )
    }
  }

  /**
   * Clear the user's Google Spreadsheet configuration
   */
  async clearSpreadsheetUrl(): Promise<void> {
    try {
      const response = await fetch(`${this.baseURL}/api/config/spreadsheet`, {
        method: 'DELETE',
        credentials: 'include', // Include cookies for authentication
      })

      if (!response.ok) {
        // Try to parse error response
        try {
          const errorData: ConfigError = await response.json()
          throw new ConfigApiError(
            errorData.error || 'UNKNOWN_ERROR',
            errorData.message || 'Failed to clear spreadsheet configuration',
            errorData.type,
            response.status
          )
        } catch (parseError) {
          // If we can't parse the error response, throw a generic error
          throw new ConfigApiError(
            'HTTP_ERROR',
            `HTTP ${response.status}: ${response.statusText}`,
            undefined,
            response.status
          )
        }
      }

      const result: SetSpreadsheetResponse = await response.json()
      if (!result.success) {
        throw new ConfigApiError(
          'API_ERROR',
          result.message || 'Failed to clear spreadsheet configuration',
          undefined,
          response.status
        )
      }
    } catch (error) {
      if (error instanceof ConfigApiError) {
        throw error
      }

      // Network or other unexpected errors
      console.error('Error clearing spreadsheet URL:', error)
      throw new ConfigApiError(
        'NETWORK_ERROR',
        'Unable to connect to the server. Please check your internet connection and try again.',
        undefined,
        0
      )
    }
  }
}

/**
 * Custom error class for configuration API errors
 */
export class ConfigApiError extends Error {
  public readonly code: string
  public readonly type?: string
  public readonly statusCode: number

  constructor(code: string, message: string, type?: string, statusCode: number = 0) {
    super(message)
    this.name = 'ConfigApiError'
    this.code = code
    this.type = type
    this.statusCode = statusCode
  }

  /**
   * Check if this is a specific type of error
   */
  isType(errorType: string): boolean {
    return this.type === errorType || this.code === errorType
  }

  /**
   * Get user-friendly error message based on error type
   */
  getUserFriendlyMessage(): string {
    switch (this.type || this.code) {
      case 'INVALID_URL':
        return 'Please enter a valid Google Spreadsheet URL. Make sure you\'re copying the full URL from your browser.'
      case 'PERMISSION_DENIED':
        return 'You don\'t have permission to access this spreadsheet. Please make sure the spreadsheet is shared with your Google account with edit permissions.'
      case 'NOT_FOUND':
        return 'Spreadsheet not found. Please check that the URL is correct and the spreadsheet exists.'
      case 'NETWORK_ERROR':
        return 'Unable to connect to the server. Please check your internet connection and try again.'
      case 'UNAUTHORIZED':
        return 'Your session has expired. Please sign in again.'
      default:
        return this.message || 'An unexpected error occurred. Please try again.'
    }
  }
}

// Create singleton instance
export const configService = new ConfigService()

// Export default instance
export default configService