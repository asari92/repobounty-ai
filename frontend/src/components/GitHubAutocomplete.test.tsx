import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { useState } from 'react';
import GitHubAutocomplete from './GitHubAutocomplete';
import type { UserSearchResult, RepoSearchResult } from '../types';

vi.mock('../api/client', () => ({
  api: {
    searchGitHub: vi.fn(),
  },
}));

const { api } = await import('../api/client');

afterEach(() => {
  vi.clearAllMocks();
  vi.useRealTimers();
});

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
    fireEvent.change(input, { target: { value: 'test/repo1' } });

    const dropdown = await screen.findByRole('listbox');
    expect(dropdown).toBeVisible();
  });

  it('normalizes pasted GitHub repo URLs into owner/repo', async () => {
    const TestWrapper = () => {
      const [value, setValue] = useState('');
      return <GitHubAutocomplete value={value} onChange={setValue} />;
    };

    render(<TestWrapper />);
    const input = screen.getByPlaceholderText('owner/repo');

    fireEvent.change(input, { target: { value: 'https://github.com/octocat/hello-world.git' } });

    expect(screen.getByDisplayValue('octocat/hello-world')).toBeVisible();
  });

  it('shows public repositories immediately after owner slash', async () => {
    vi.mocked(api.searchGitHub).mockResolvedValue([
      { owner: 'octocat', name: 'hello-world' },
      { owner: 'octocat', name: 'docs' },
    ]);

    const TestWrapper = () => {
      const [value, setValue] = useState('octocat/');
      return <GitHubAutocomplete value={value} onChange={setValue} />;
    };

    render(<TestWrapper />);
    expect(await screen.findByRole('option', { name: /octocat\/hello-world/i })).toBeVisible();
    expect(api.searchGitHub).toHaveBeenCalledWith('octocat/');
  });

  it('narrows repository suggestions after typing a prefix', async () => {
    vi.mocked(api.searchGitHub).mockResolvedValue([{ owner: 'octocat', name: 'hello-world' }]);

    const TestWrapper = () => {
      const [value, setValue] = useState('octocat/he');
      return <GitHubAutocomplete value={value} onChange={setValue} />;
    };

    render(<TestWrapper />);
    const option = await screen.findByRole('option', { name: /octocat\/hello-world/i });
    fireEvent.click(option);

    expect(screen.getByDisplayValue('octocat/hello-world')).toBeVisible();
    expect(api.searchGitHub).toHaveBeenCalledWith('octocat/he');
  });

  it('shows empty-state text when owner has no public repositories', async () => {
    vi.mocked(api.searchGitHub).mockResolvedValue([]);

    const TestWrapper = () => {
      const [value, setValue] = useState('octocat/');
      return <GitHubAutocomplete value={value} onChange={setValue} />;
    };

    render(<TestWrapper />);
    expect(await screen.findByText('No public repositories found')).toBeVisible();
  });

  it('shows request error text instead of crashing', async () => {
    vi.mocked(api.searchGitHub).mockRejectedValue(new Error('boom'));

    const TestWrapper = () => {
      const [value, setValue] = useState('octocat/');
      return <GitHubAutocomplete value={value} onChange={setValue} />;
    };

    render(<TestWrapper />);
    expect(await screen.findByText('Could not load GitHub results')).toBeVisible();
  });
});
