/**
 * SyncService - API client for manual sync operations
 * Provides methods to trigger manual sync and get queue status
 */

const API_BASE = process.env.NODE_ENV === 'production' 
  ? '/api' 
  : 'http://localhost:8080/api';

/**
 * Response type for successful sync trigger
 * @typedef {Object} SyncResponse
 * @property {boolean} success - Whether the sync was triggered successfully
 * @property {string} message - Success message
 * @property {string} trace_id - Unique trace ID for the sync job
 * @property {number} estimated_completion_seconds - Estimated time for completion
 */

/**
 * Error response type for sync operations
 * @typedef {Object} SyncErrorResponse
 * @property {string} error - Error code
 * @property {string} message - Error message
 * @property {string} type - Error type
 */

/**
 * Queue status response type
 * @typedef {Object} QueueStatusResponse
 * @property {number} queue_length - Current number of jobs in queue
 * @property {string} queue_name - Name of the job queue
 * @property {string} status_time - Timestamp of status check
 * @property {string} health_status - Redis health status
 */

/**
 * Custom error class for sync API errors
 */
export class SyncError extends Error {
  constructor(message, code, type, status) {
    super(message);
    this.name = 'SyncError';
    this.code = code;
    this.type = type;
    this.status = status;
  }
}

/**
 * Makes an authenticated API request
 * @param {string} endpoint - API endpoint path
 * @param {RequestInit} options - Fetch options
 * @returns {Promise<Response>} Fetch response
 * @throws {SyncError} If request fails or returns error status
 */
async function makeAuthenticatedRequest(endpoint, options = {}) {
  const token = localStorage.getItem('authToken');
  
  if (!token) {
    throw new SyncError('Authentication required', 'UNAUTHORIZED', 'AUTH_ERROR', 401);
  }

  const url = `${API_BASE}${endpoint}`;
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`,
    ...options.headers,
  };

  try {
    const response = await fetch(url, {
      ...options,
      headers,
    });

    // Handle different response statuses
    if (response.status === 401) {
      // Token might be expired, clear it
      localStorage.removeItem('authToken');
      throw new SyncError('Authentication required', 'UNAUTHORIZED', 'AUTH_ERROR', 401);
    }

    if (response.status === 403) {
      throw new SyncError('Access forbidden', 'FORBIDDEN', 'AUTH_ERROR', 403);
    }

    if (response.status === 404) {
      throw new SyncError('User not found or not configured', 'USER_NOT_FOUND', 'USER_ERROR', 404);
    }

    if (response.status === 503) {
      throw new SyncError('Sync service temporarily unavailable', 'SERVICE_UNAVAILABLE', 'SERVICE_ERROR', 503);
    }

    // For error responses, parse the error body
    if (!response.ok) {
      try {
        const errorData = await response.json();
        throw new SyncError(
          errorData.message || 'An unexpected error occurred',
          errorData.error || 'UNKNOWN_ERROR',
          errorData.type || 'API_ERROR',
          response.status
        );
      } catch (parseError) {
        // If we can't parse the error response, throw a generic error
        throw new SyncError(
          `HTTP ${response.status}: ${response.statusText}`,
          'HTTP_ERROR',
          'NETWORK_ERROR',
          response.status
        );
      }
    }

    return response;
  } catch (error) {
    // Network errors or other fetch failures
    if (error instanceof SyncError) {
      throw error;
    }
    
    throw new SyncError(
      'Network error: Unable to connect to sync service',
      'NETWORK_ERROR',
      'NETWORK_ERROR',
      0
    );
  }
}

/**
 * SyncService class providing manual sync operations
 */
export class SyncService {
  /**
   * Triggers a manual sync for the authenticated user
   * @returns {Promise<SyncResponse>} Sync response with trace ID
   * @throws {SyncError} If sync trigger fails
   */
  static async triggerManualSync() {
    console.log('🔄 Triggering manual sync...');
    
    try {
      const response = await makeAuthenticatedRequest('/sync', {
        method: 'POST',
      });

      const data = await response.json();
      
      console.log('✅ Manual sync triggered successfully:', {
        trace_id: data.trace_id,
        estimated_completion: data.estimated_completion_seconds
      });

      return data;
    } catch (error) {
      console.error('❌ Failed to trigger manual sync:', error);
      throw error;
    }
  }

  /**
   * Gets the current queue status (for debugging)
   * @returns {Promise<QueueStatusResponse>} Queue status information
   * @throws {SyncError} If status retrieval fails
   */
  static async getQueueStatus() {
    console.log('📊 Getting queue status...');
    
    try {
      const response = await makeAuthenticatedRequest('/sync/status', {
        method: 'GET',
      });

      const data = await response.json();
      
      console.log('📊 Queue status retrieved:', data);

      return data;
    } catch (error) {
      console.error('❌ Failed to get queue status:', error);
      throw error;
    }
  }

  /**
   * Checks if manual sync is available (Redis configured)
   * This is a simple check that tries to get queue status
   * @returns {Promise<boolean>} Whether manual sync is available
   */
  static async isManualSyncAvailable() {
    try {
      await this.getQueueStatus();
      return true;
    } catch (error) {
      // If queue status fails, manual sync is probably not available
      return false;
    }
  }
}

export default SyncService;