import { describe, it, expect } from 'vitest';
import { getDefaultDeadline } from './CreateCampaign';

describe('CreateCampaign', () => {
  describe('getDefaultDeadline', () => {
    it('should return now + 1 minute in correct format', () => {
      const result = getDefaultDeadline();
      const now = new Date();
      const resultDate = new Date(result);
      const expectedDate = new Date(now.getTime() + 60000);

      expect(result).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/);
      expect(resultDate.getFullYear()).toBe(expectedDate.getFullYear());
      expect(resultDate.getMonth()).toBe(expectedDate.getMonth());
      expect(resultDate.getDate()).toBe(expectedDate.getDate());
      expect(resultDate.getHours()).toBe(expectedDate.getHours());
      expect(resultDate.getMinutes()).toBe(expectedDate.getMinutes());
    });
  });
});
