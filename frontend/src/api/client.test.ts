import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { api } from './client';

describe('api.searchGitHub', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', {
      getItem: vi.fn(() => null),
      setItem: vi.fn(() => {}),
      removeItem: vi.fn(() => {}),
      clear: vi.fn(() => {}),
    });

    vi.stubGlobal('fetch', vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve([
            { login: 'testuser', avatar_url: 'https://example.com/avatar.png' },
            { name: 'testrepo', owner: 'testowner' },
          ]),
      })
    ));
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should return search results', async () => {
    const results = await api.searchGitHub('test');
    expect(results).toBeDefined();
    expect(results.length).toBeGreaterThan(0);
  });
});
