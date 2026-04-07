import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { vi } from 'vitest';
import { useState } from 'react';
import GitHubAutocomplete from './GitHubAutocomplete';
import type { UserSearchResult, RepoSearchResult } from '../types';

vi.mock('../api/client', () => ({
  api: {
    searchGitHub: vi.fn(),
  },
}));

const { api } = await import('../api/client');

describe('GitHubAutocomplete', () => {
  it('should show dropdown when typing owner', async () => {
    const mockResults: UserSearchResult[] = [
      { login: 'testuser', avatar_url: 'https://example.com/avatar.png' },
    ];
    vi.mocked(api.searchGitHub).mockResolvedValue(mockResults);

    const TestWrapper = () => {
      const [value, setValue] = useState('');
      return <GitHubAutocomplete value={value} onChange={setValue} />;
    };

    const { getByPlaceholderText } = render(<TestWrapper />);
    const input = getByPlaceholderText('owner/repo');
    fireEvent.change(input, { target: { value: 'test' } });

    await waitFor(() => {
      const dropdown = screen.getByRole('listbox');
      expect(dropdown).toBeVisible();
    });
  });

  it('should show repositories when slash is entered', async () => {
    const mockResults: RepoSearchResult[] = [{ name: 'repo1', owner: 'testuser' }];
    vi.mocked(api.searchGitHub).mockResolvedValue(mockResults);

    const TestWrapper = () => {
      const [value, setValue] = useState('');
      return <GitHubAutocomplete value={value} onChange={setValue} />;
    };

    const { getByPlaceholderText } = render(<TestWrapper />);
    const input = getByPlaceholderText('owner/repo');
    fireEvent.change(input, { target: { value: 'test/' } });

    await waitFor(() => {
      const dropdown = screen.getByRole('listbox');
      expect(dropdown).toBeVisible();
    });
  });
});
