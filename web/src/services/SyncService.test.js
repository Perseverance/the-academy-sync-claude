/**
 * SyncService Tests
 * Tests for the manual sync API client
 */

import { SyncService, SyncError } from './SyncService';

// Mock fetch for testing
global.fetch = jest.fn();

// Mock localStorage
const localStorageMock = {
  getItem: jest.fn(),
  removeItem: jest.fn(),
};
global.localStorage = localStorageMock;

describe('SyncService', () => {
  beforeEach(() => {
    // Reset all mocks before each test
    fetch.mockClear();
    localStorageMock.getItem.mockClear();
    localStorageMock.removeItem.mockClear();
    
    // Setup console mocks to avoid test output noise
    jest.spyOn(console, 'log').mockImplementation(() => {});
    jest.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    // Restore console methods
    console.log.mockRestore();
    console.error.mockRestore();
  });

  describe('triggerManualSync', () => {
    it('should successfully trigger manual sync', async () => {
      // Mock successful response
      const mockResponse = {
        success: true,
        message: 'Manual sync triggered successfully',
        trace_id: 'test-trace-123',
        estimated_completion_seconds: 60
      };

      localStorageMock.getItem.mockReturnValue('mock-auth-token');
      fetch.mockResolvedValueOnce({
        ok: true,
        status: 202,
        json: async () => mockResponse
      });

      const result = await SyncService.triggerManualSync();

      expect(fetch).toHaveBeenCalledWith(
        expect.stringContaining('/api/sync'),
        expect.objectContaining({
          method: 'POST',
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
            'Authorization': 'Bearer mock-auth-token'
          })
        })
      );

      expect(result).toEqual(mockResponse);
    });

    it('should throw SyncError when user not authenticated', async () => {
      localStorageMock.getItem.mockReturnValue(null);

      const error = await SyncService.triggerManualSync().catch(e => e);
      
      expect(error).toBeInstanceOf(SyncError);
      expect(error.message).toContain('Authentication required');
    });

    it('should handle 401 unauthorized response', async () => {
      localStorageMock.getItem.mockReturnValue('expired-token');
      fetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        statusText: 'Unauthorized'
      });

      await expect(SyncService.triggerManualSync()).rejects.toThrow(SyncError);
      
      // Should remove token from localStorage
      expect(localStorageMock.removeItem).toHaveBeenCalledWith('authToken');
    });

    it('should handle 400 user not configured response', async () => {
      const errorResponse = {
        error: 'USER_NOT_CONFIGURED',
        message: 'User is not fully configured for automation',
        type: 'USER_NOT_CONFIGURED'
      };

      localStorageMock.getItem.mockReturnValue('valid-token');
      fetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        json: async () => errorResponse
      });

      const error = await SyncService.triggerManualSync().catch(e => e);
      
      expect(error).toBeInstanceOf(SyncError);
      expect(error.code).toBe('USER_NOT_CONFIGURED');
      expect(error.status).toBe(400);
    });

    it('should handle 503 service unavailable response', async () => {
      localStorageMock.getItem.mockReturnValue('valid-token');
      fetch.mockResolvedValueOnce({
        ok: false,
        status: 503,
        statusText: 'Service Unavailable'
      });

      const error = await SyncService.triggerManualSync().catch(e => e);
      
      expect(error).toBeInstanceOf(SyncError);
      expect(error.type).toBe('SERVICE_ERROR');
      expect(error.status).toBe(503);
    });

    it('should handle network errors', async () => {
      localStorageMock.getItem.mockReturnValue('valid-token');
      fetch.mockRejectedValueOnce(new Error('Network connection failed'));

      const error = await SyncService.triggerManualSync().catch(e => e);
      
      expect(error).toBeInstanceOf(SyncError);
      expect(error.type).toBe('NETWORK_ERROR');
      expect(error.message).toContain('Network error');
    });
  });

  describe('getQueueStatus', () => {
    it('should successfully get queue status', async () => {
      const mockStatus = {
        queue_length: 5,
        queue_name: 'jobs_queue',
        health_status: 'healthy',
        status_time: '2024-01-15T10:30:00Z'
      };

      localStorageMock.getItem.mockReturnValue('valid-token');
      fetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => mockStatus
      });

      const result = await SyncService.getQueueStatus();

      expect(fetch).toHaveBeenCalledWith(
        expect.stringContaining('/api/sync/status'),
        expect.objectContaining({
          method: 'GET',
          headers: expect.objectContaining({
            'Authorization': 'Bearer valid-token'
          })
        })
      );

      expect(result).toEqual(mockStatus);
    });

    it('should throw SyncError when unauthorized', async () => {
      localStorageMock.getItem.mockReturnValue(null);

      await expect(SyncService.getQueueStatus()).rejects.toThrow(SyncError);
    });
  });

  describe('isManualSyncAvailable', () => {
    it('should return true when queue status is available', async () => {
      localStorageMock.getItem.mockReturnValue('valid-token');
      fetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ queue_length: 0 })
      });

      const result = await SyncService.isManualSyncAvailable();

      expect(result).toBe(true);
    });

    it('should return false when queue status fails', async () => {
      localStorageMock.getItem.mockReturnValue('valid-token');
      fetch.mockResolvedValueOnce({
        ok: false,
        status: 503
      });

      const result = await SyncService.isManualSyncAvailable();

      expect(result).toBe(false);
    });
  });
});

describe('SyncError', () => {
  it('should create error with all properties', () => {
    const error = new SyncError('Test message', 'TEST_CODE', 'TEST_TYPE', 400);

    expect(error.name).toBe('SyncError');
    expect(error.message).toBe('Test message');
    expect(error.code).toBe('TEST_CODE');
    expect(error.type).toBe('TEST_TYPE');
    expect(error.status).toBe(400);
    expect(error instanceof Error).toBe(true);
  });
});