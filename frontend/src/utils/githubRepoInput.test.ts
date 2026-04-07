import { describe, expect, it, vi } from 'vitest';
import { getGitHubSearchState, normalizeGitHubRepoInput } from './githubRepoInput';

describe('normalizeGitHubRepoInput', () => {
  it('keeps owner/repo as-is except trimming whitespace', () => {
    expect(normalizeGitHubRepoInput(' octocat/hello-world ')).toBe('octocat/hello-world');
  });

  it('normalizes https GitHub repository URLs', () => {
    expect(normalizeGitHubRepoInput('https://github.com/octocat/hello-world')).toBe(
      'octocat/hello-world'
    );
  });

  it('normalizes https GitHub repository URLs with .git suffix', () => {
    expect(normalizeGitHubRepoInput('https://github.com/octocat/hello-world.git')).toBe(
      'octocat/hello-world'
    );
  });

  it('normalizes SSH GitHub repository URLs with .git suffix', () => {
    expect(normalizeGitHubRepoInput('git@github.com:octocat/hello-world.git')).toBe(
      'octocat/hello-world'
    );
  });

  it('does not rewrite unsupported nested GitHub URLs', () => {
    expect(normalizeGitHubRepoInput('https://github.com/octocat/hello-world/issues/1')).toBe(
      'https://github.com/octocat/hello-world/issues/1'
    );
  });
});

describe('getGitHubSearchState', () => {
  it('treats owner input as user search', () => {
    expect(getGitHubSearchState('octo')).toMatchObject({
      mode: 'users',
      query: 'octo',
      owner: 'octo',
      repoPrefix: '',
      isCompleteRepo: false,
    });
  });

  it('treats owner slash as repo search with empty prefix', () => {
    expect(getGitHubSearchState('octocat/')).toMatchObject({
      mode: 'repos',
      query: 'octocat/',
      owner: 'octocat',
      repoPrefix: '',
      isCompleteRepo: false,
    });
  });

  it('treats owner prefix as repo search', () => {
    expect(getGitHubSearchState('octocat/he')).toMatchObject({
      mode: 'repos',
      query: 'octocat/he',
      owner: 'octocat',
      repoPrefix: 'he',
      isCompleteRepo: true,
    });
  });

  it('marks multiple slashes as invalid', () => {
    expect(getGitHubSearchState('octocat/hello/world').mode).toBe('invalid');
  });
});
