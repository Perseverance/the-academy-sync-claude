// SyncService handles manual sync operations with error context preservation
export class SyncError extends Error {
  public readonly code: string;
  public readonly type: string;
  public readonly status: number;
  public readonly cause: Error | null;

  constructor(message: string, code: string, type: string, status: number, cause: Error | null = null) {
    super(message);
    this.name = 'SyncError';
    this.code = code;
    this.type = type;
    this.status = status;
    this.cause = cause;
  }
}

export interface SyncResponse {
  status: string;
  message: string;
}

export interface SyncStatusResponse {
  eligible: boolean;
  reason?: string;
}

class SyncService {
  private readonly baseURL: string;

  constructor() {
    // Use environment variable for API URL, fallback to localhost
    this.baseURL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
  }

  /**
   * Triggers a manual sync for the authenticated user
   * @returns Promise that resolves when sync is queued successfully
   * @throws SyncError when sync cannot be triggered
   */
  async triggerManualSync(): Promise<SyncResponse> {
    try {
      const response = await fetch(`${this.baseURL}/api/sync`, {
        method: 'POST',
        credentials: 'include', // Include cookies for session management
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        await this.handleErrorResponse(response);
      }

      const data: SyncResponse = await response.json();
      return data;
    } catch (error) {
      if (error instanceof SyncError) {
        throw error;
      }
      
      // Network or other unexpected errors
      throw new SyncError(
        'Failed to connect to sync service. Please check your connection and try again.',
        'NETWORK_ERROR',
        'connection',
        0,
        error instanceof Error ? error : null
      );
    }
  }

  /**
   * Gets the sync eligibility status for the authenticated user
   * @returns Promise that resolves with sync eligibility information
   * @throws SyncError when status cannot be retrieved
   */
  async getSyncStatus(): Promise<SyncStatusResponse> {
    try {
      const response = await fetch(`${this.baseURL}/api/sync/status`, {
        method: 'GET',
        credentials: 'include', // Include cookies for session management
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        await this.handleErrorResponse(response);
      }

      const data: SyncStatusResponse = await response.json();
      return data;
    } catch (error) {
      if (error instanceof SyncError) {
        throw error;
      }
      
      // Network or other unexpected errors
      throw new SyncError(
        'Failed to get sync status. Please check your connection and try again.',
        'NETWORK_ERROR',
        'connection',
        0,
        error instanceof Error ? error : null
      );
    }
  }

  /**
   * Handles error responses from the API
   * @param response The failed response
   * @throws SyncError with appropriate details
   */
  private async handleErrorResponse(response: Response): Promise<never> {
    let errorData: any;
    try {
      errorData = await response.json();
    } catch {
      // If we can't parse the error response, create a generic error
      throw new SyncError(
        `HTTP ${response.status}: ${response.statusText}`,
        'HTTP_ERROR',
        'http',
        response.status
      );
    }

    const message = errorData.error || errorData.message || 'An unexpected error occurred';
    
    // Determine error type based on status code
    let errorType: string;
    let errorCode: string;

    switch (response.status) {
      case 400:
        errorType = 'validation';
        errorCode = 'VALIDATION_ERROR';
        break;
      case 401:
        errorType = 'authentication';
        errorCode = 'AUTH_ERROR';
        break;
      case 403:
        errorType = 'authorization';
        errorCode = 'FORBIDDEN';
        break;
      case 404:
        errorType = 'not_found';
        errorCode = 'NOT_FOUND';
        break;
      case 503:
        errorType = 'service_unavailable';
        errorCode = 'SERVICE_UNAVAILABLE';
        break;
      case 500:
      default:
        errorType = 'server';
        errorCode = 'SERVER_ERROR';
        break;
    }

    throw new SyncError(message, errorCode, errorType, response.status);
  }
}

// Create and export singleton instance
export const syncService = new SyncService();

// Export default instance
export default syncService;