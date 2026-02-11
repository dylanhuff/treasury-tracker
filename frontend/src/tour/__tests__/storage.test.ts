import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { getTourCompleted, setTourCompleted } from '../storage';

const TOUR_STORAGE_KEY = 'treasuryApp_tourCompleted';

describe('Tour Storage Utilities', () => {
  // Save original localStorage for restoration
  let originalLocalStorage: Storage;

  beforeEach(() => {
    // Clear localStorage before each test
    localStorage.clear();
    // Save original localStorage
    originalLocalStorage = window.localStorage;
  });

  afterEach(() => {
    // Restore original localStorage after each test
    Object.defineProperty(window, 'localStorage', {
      value: originalLocalStorage,
      writable: true,
      configurable: true,
    });
  });

  describe('getTourCompleted()', () => {
    it('should return false when no value is stored (first visit)', () => {
      const result = getTourCompleted();
      expect(result).toBe(false);
    });

    it('should return true when tour is marked as completed', () => {
      localStorage.setItem(TOUR_STORAGE_KEY, 'true');
      const result = getTourCompleted();
      expect(result).toBe(true);
    });

    it('should return false when tour is marked as not completed', () => {
      localStorage.setItem(TOUR_STORAGE_KEY, 'false');
      const result = getTourCompleted();
      expect(result).toBe(false);
    });

    it('should return false when stored value is invalid', () => {
      localStorage.setItem(TOUR_STORAGE_KEY, 'invalid-value');
      const result = getTourCompleted();
      expect(result).toBe(false);
    });

    it('should return false when localStorage is unavailable', () => {
      // Mock localStorage as undefined (e.g., private browsing mode)
      Object.defineProperty(window, 'localStorage', {
        value: undefined,
        writable: true,
        configurable: true,
      });

      const result = getTourCompleted();
      expect(result).toBe(false);
    });

    it('should handle localStorage.getItem throwing an error', () => {
      // Mock getItem to throw an error (e.g., SecurityError)
      const mockGetItem = vi.fn(() => {
        throw new Error('SecurityError: localStorage access denied');
      });

      // Create a full mock localStorage object with getItem that throws
      const mockStorage = {
        getItem: mockGetItem,
        setItem: vi.fn(),
        removeItem: vi.fn(),
        clear: vi.fn(),
        key: vi.fn(),
        length: 0,
      };

      Object.defineProperty(window, 'localStorage', {
        value: mockStorage,
        writable: true,
        configurable: true,
      });

      // Should not throw, should return false and log error
      const result = getTourCompleted();
      expect(result).toBe(false);
      expect(mockGetItem).toHaveBeenCalledWith(TOUR_STORAGE_KEY);
    });

    it('should return false when localStorage throws SecurityError', () => {
      // Create a mock that throws SecurityError
      const mockStorage = {
        getItem: vi.fn(() => {
          const error = new Error('SecurityError');
          error.name = 'SecurityError';
          throw error;
        }),
      };

      Object.defineProperty(window, 'localStorage', {
        value: mockStorage,
        writable: true,
        configurable: true,
      });

      const result = getTourCompleted();
      expect(result).toBe(false);
    });
  });

  describe('setTourCompleted()', () => {
    it('should store true when tour is completed', () => {
      setTourCompleted(true);
      const stored = localStorage.getItem(TOUR_STORAGE_KEY);
      expect(stored).toBe('true');
    });

    it('should store false when tour is not completed', () => {
      setTourCompleted(false);
      const stored = localStorage.getItem(TOUR_STORAGE_KEY);
      expect(stored).toBe('false');
    });

    it('should overwrite existing value', () => {
      // First set to true
      setTourCompleted(true);
      expect(localStorage.getItem(TOUR_STORAGE_KEY)).toBe('true');

      // Then set to false
      setTourCompleted(false);
      expect(localStorage.getItem(TOUR_STORAGE_KEY)).toBe('false');

      // Then set back to true
      setTourCompleted(true);
      expect(localStorage.getItem(TOUR_STORAGE_KEY)).toBe('true');
    });

    it('should handle localStorage being unavailable gracefully', () => {
      // Mock localStorage as undefined
      Object.defineProperty(window, 'localStorage', {
        value: undefined,
        writable: true,
        configurable: true,
      });

      // Should not throw
      expect(() => setTourCompleted(true)).not.toThrow();
    });

    it('should handle localStorage.setItem throwing QuotaExceededError', () => {
      // Mock setItem to throw QuotaExceededError
      const mockSetItem = vi.fn(() => {
        const error = new Error('QuotaExceededError');
        error.name = 'QuotaExceededError';
        throw error;
      });

      // Create a full mock localStorage object with setItem that throws
      const mockStorage = {
        getItem: vi.fn(),
        setItem: mockSetItem,
        removeItem: vi.fn(),
        clear: vi.fn(),
        key: vi.fn(),
        length: 0,
      };

      Object.defineProperty(window, 'localStorage', {
        value: mockStorage,
        writable: true,
        configurable: true,
      });

      // Should not throw, should handle gracefully
      expect(() => setTourCompleted(true)).not.toThrow();
      expect(mockSetItem).toHaveBeenCalledWith(TOUR_STORAGE_KEY, 'true');
    });

    it('should handle localStorage.setItem throwing generic error', () => {
      const mockSetItem = vi.fn(() => {
        throw new Error('Generic storage error');
      });

      Object.defineProperty(window.localStorage, 'setItem', {
        value: mockSetItem,
        writable: true,
        configurable: true,
      });

      // Should not throw
      expect(() => setTourCompleted(true)).not.toThrow();
    });
  });

  describe('Integration scenarios', () => {
    it('should handle complete first-visit flow', () => {
      // First visit - no stored value
      expect(getTourCompleted()).toBe(false);

      // User completes tour
      setTourCompleted(true);
      expect(getTourCompleted()).toBe(true);

      // Verify persistence
      expect(localStorage.getItem(TOUR_STORAGE_KEY)).toBe('true');
    });

    it('should handle tour skip flow', () => {
      // User starts tour
      expect(getTourCompleted()).toBe(false);

      // User skips tour (still marked as completed to prevent auto-restart)
      setTourCompleted(true);
      expect(getTourCompleted()).toBe(true);
    });

    it('should handle manual tour restart flow', () => {
      // Tour already completed
      setTourCompleted(true);
      expect(getTourCompleted()).toBe(true);

      // User manually restarts tour (completion status remains)
      // Tour component would check this and allow restart despite status
      expect(getTourCompleted()).toBe(true);

      // Tour completes again
      setTourCompleted(true);
      expect(getTourCompleted()).toBe(true);
    });

    it('should maintain state across multiple operations', () => {
      // Initial state
      expect(getTourCompleted()).toBe(false);

      // Set to true
      setTourCompleted(true);
      expect(getTourCompleted()).toBe(true);

      // Set to false
      setTourCompleted(false);
      expect(getTourCompleted()).toBe(false);

      // Set back to true
      setTourCompleted(true);
      expect(getTourCompleted()).toBe(true);
    });
  });

  describe('Edge cases and error handling', () => {
    it('should handle window being undefined (SSR scenario)', () => {
      // Save original window
      const originalWindow = globalThis.window;

      // Mock window as undefined (server-side rendering)
      // @ts-expect-error - Intentionally setting to undefined for test
      globalThis.window = undefined;

      const result = getTourCompleted();
      expect(result).toBe(false);

      // Restore window
      globalThis.window = originalWindow;
    });

    it('should handle concurrent reads and writes', () => {
      // Simulate rapid successive operations
      setTourCompleted(true);
      expect(getTourCompleted()).toBe(true);

      setTourCompleted(false);
      expect(getTourCompleted()).toBe(false);

      setTourCompleted(true);
      expect(getTourCompleted()).toBe(true);

      // All operations should complete successfully
      expect(localStorage.getItem(TOUR_STORAGE_KEY)).toBe('true');
    });

    it('should handle corrupted localStorage data', () => {
      // Simulate corrupted/unexpected data
      localStorage.setItem(TOUR_STORAGE_KEY, '{"invalid": "json"}');
      expect(getTourCompleted()).toBe(false);

      localStorage.setItem(TOUR_STORAGE_KEY, 'null');
      expect(getTourCompleted()).toBe(false);

      localStorage.setItem(TOUR_STORAGE_KEY, '123');
      expect(getTourCompleted()).toBe(false);

      localStorage.setItem(TOUR_STORAGE_KEY, '');
      expect(getTourCompleted()).toBe(false);
    });

    it('should handle very long values gracefully', () => {
      // Even though we only store boolean, test handling of unexpected long values
      const longValue = 'x'.repeat(10000);
      localStorage.setItem(TOUR_STORAGE_KEY, longValue);

      // Should return false for invalid value
      expect(getTourCompleted()).toBe(false);
    });
  });
});
