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

function renderAutocomplete(initialValue = '') {
  const onChange = vi.fn();
  const TestWrapper = () => {
    const [value, setValue] = useState(initialValue);
    const handleChange = (v: string) => {
      onChange(v);
      setValue(v);
    };
    return <GitHubAutocomplete value={value} onChange={handleChange} />;
  };
  const result = render(<TestWrapper />);
  const input = screen.getByPlaceholderText('owner/repo');
  return { ...result, input, onChange };
}

describe('GitHubAutocomplete', () => {
  it('shows user results when typing owner name', async () => {
    vi.mocked(api.searchGitHub).mockResolvedValue([
      { login: 'testuser', avatar_url: 'https://example.com/avatar.png' },
    ] as UserSearchResult[]);

    const { input } = renderAutocomplete();
    fireEvent.change(input, { target: { value: 'test' } });

    const option = await screen.findByRole('option');
    expect(option).toBeVisible();
    expect(screen.getByText('testuser')).toBeVisible();
    expect(api.searchGitHub).toHaveBeenCalledWith('test');
  });

  it('sets input to owner/ after selecting owner (not just /)', async () => {
    vi.mocked(api.searchGitHub).mockResolvedValue([
      { login: 'octocat', avatar_url: 'https://example.com/avatar.png' },
    ] as UserSearchResult[]);

    const { input, onChange } = renderAutocomplete();
    fireEvent.change(input, { target: { value: 'octo' } });

    const option = await screen.findByRole('option');
    fireEvent.click(option);

    expect(onChange).toHaveBeenCalledWith('octocat/');
    expect(screen.getByDisplayValue('octocat/')).toBeVisible();
  });

  it('shows repos after owner selection without showing no-results', async () => {
    vi.mocked(api.searchGitHub)
      .mockResolvedValueOnce([
        { login: 'octocat', avatar_url: 'https://example.com/a.png' },
      ] as UserSearchResult[])
      .mockResolvedValueOnce([
        { owner: 'octocat', name: 'hello-world' },
        { owner: 'octocat', name: 'linguist' },
      ] as RepoSearchResult[]);

    const { input } = renderAutocomplete();
    fireEvent.change(input, { target: { value: 'octo' } });

    const ownerOption = await screen.findByRole('option');
    fireEvent.click(ownerOption);

    expect(await screen.findByRole('option', { name: /octocat\/hello-world/i })).toBeVisible();
    expect(screen.getByRole('option', { name: /octocat\/linguist/i })).toBeVisible();
    expect(api.searchGitHub).toHaveBeenCalledWith('octocat/');
  });

  it('filters repos by prefix within selected owner', async () => {
    vi.mocked(api.searchGitHub)
      .mockResolvedValueOnce([
        { owner: 'octocat', name: 'hello-world' },
        { owner: 'octocat', name: 'linguist' },
      ] as RepoSearchResult[])
      .mockResolvedValueOnce([
        { owner: 'octocat', name: 'hello-world' },
      ] as RepoSearchResult[]);

    const { input } = renderAutocomplete('octocat/');
    await screen.findByRole('option', { name: /octocat\/hello-world/i });

    fireEvent.change(input, { target: { value: 'octocat/he' } });

    await waitFor(() => {
      expect(api.searchGitHub).toHaveBeenCalledWith('octocat/he');
    });
  });

  it('selects repo, sets exact owner/repo, and closes dropdown', async () => {
    vi.mocked(api.searchGitHub).mockResolvedValue([
      { owner: 'octocat', name: 'hello-world' },
    ] as RepoSearchResult[]);

    const { input, onChange } = renderAutocomplete('octocat/he');
    const option = await screen.findByRole('option', { name: /octocat\/hello-world/i });
    fireEvent.click(option);

    expect(onChange).toHaveBeenCalledWith('octocat/hello-world');
    expect(screen.getByDisplayValue('octocat/hello-world')).toBeVisible();

    await waitFor(() => {
      expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    });
  });

  it('does not show "No matching repositories" after repo selection', async () => {
    vi.mocked(api.searchGitHub)
      .mockResolvedValueOnce([
        { owner: 'octocat', name: 'hello-world' },
      ] as RepoSearchResult[])
      .mockResolvedValue([]);

    const { input } = renderAutocomplete('octocat/he');
    const option = await screen.findByRole('option', { name: /octocat\/hello-world/i });
    fireEvent.click(option);

    await waitFor(() => {
      expect(screen.queryByText(/No matching repositories/)).not.toBeInTheDocument();
      expect(screen.queryByText(/No public repositories found/)).not.toBeInTheDocument();
    });
  });

  it('validates exact owner/repo paste via API', async () => {
    vi.mocked(api.searchGitHub).mockResolvedValue([
      { owner: 'octocat', name: 'hello-world' },
    ] as RepoSearchResult[]);

    renderAutocomplete();
    const input = screen.getByPlaceholderText('owner/repo');
    fireEvent.change(input, { target: { value: 'octocat/hello-world' } });

    await waitFor(() => {
      expect(api.searchGitHub).toHaveBeenCalledWith('octocat/hello-world');
    });
  });

  it('does not set selected state for invalid paste', async () => {
    vi.mocked(api.searchGitHub).mockResolvedValue([]);

    const { onChange } = renderAutocomplete();
    const input = screen.getByPlaceholderText('owner/repo');
    fireEvent.change(input, { target: { value: 'nonexistent/repo123' } });

    await screen.findByText('No matching repositories');
    expect(onChange).toHaveBeenCalledWith('nonexistent/repo123');
  });

  it('normalizes pasted GitHub URL to owner/repo', async () => {
    vi.mocked(api.searchGitHub).mockResolvedValue([
      { owner: 'octocat', name: 'hello-world' },
    ] as RepoSearchResult[]);

    const { input } = renderAutocomplete();
    fireEvent.change(input, { target: { value: 'https://github.com/octocat/hello-world.git' } });
    expect(screen.getByDisplayValue('octocat/hello-world')).toBeVisible();
  });

  it('returns to user search when owner part is edited', async () => {
    vi.mocked(api.searchGitHub)
      .mockResolvedValueOnce([
        { owner: 'octocat', name: 'hello-world' },
      ] as RepoSearchResult[])
      .mockResolvedValueOnce([
        { login: 'different', avatar_url: 'https://example.com/a.png' },
      ] as UserSearchResult[]);

    const { input } = renderAutocomplete('octocat/');
    await screen.findByRole('option');

    fireEvent.change(input, { target: { value: 'different' } });

    await waitFor(() => {
      expect(api.searchGitHub).toHaveBeenCalledWith('different');
    });
  });

  it('shows empty-state for owner with no repos', async () => {
    vi.mocked(api.searchGitHub).mockResolvedValue([]);

    renderAutocomplete('octocat/');
    expect(await screen.findByText('No public repositories found')).toBeVisible();
  });

  it('shows error state on API failure', async () => {
    vi.mocked(api.searchGitHub).mockRejectedValue(new Error('boom'));

    renderAutocomplete('octocat/');
    expect(await screen.findByText('Could not load GitHub results')).toBeVisible();
  });

  it('shows no-results for repo prefix with no matches', async () => {
    vi.mocked(api.searchGitHub).mockResolvedValue([]);

    renderAutocomplete('octocat/nonexistent');
    expect(await screen.findByText('No matching repositories')).toBeVisible();
  });
});
