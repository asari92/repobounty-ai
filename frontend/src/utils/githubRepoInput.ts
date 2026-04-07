const REPO_PATH = /^([A-Za-z0-9._-]+)\/([A-Za-z0-9._-]+)$/;
const HTTPS_REPO_URL =
  /^https?:\/\/github\.com\/([A-Za-z0-9._-]+)\/([A-Za-z0-9._-]+?)(?:\.git)?\/?$/i;
const SSH_REPO_URL = /^git@github\.com:([A-Za-z0-9._-]+)\/([A-Za-z0-9._-]+?)(?:\.git)?$/i;

export type GitHubSearchMode = 'none' | 'users' | 'repos' | 'invalid';

export interface GitHubSearchState {
  normalizedValue: string;
  mode: GitHubSearchMode;
  query: string;
  owner: string;
  repoPrefix: string;
  isCompleteRepo: boolean;
}

export function normalizeGitHubRepoInput(raw: string): string {
  const trimmed = raw.trim();

  const httpsMatch = trimmed.match(HTTPS_REPO_URL);
  if (httpsMatch) {
    return `${httpsMatch[1]}/${httpsMatch[2]}`;
  }

  const sshMatch = trimmed.match(SSH_REPO_URL);
  if (sshMatch) {
    return `${sshMatch[1]}/${sshMatch[2]}`;
  }

  return trimmed;
}

export function getGitHubSearchState(raw: string): GitHubSearchState {
  const normalizedValue = normalizeGitHubRepoInput(raw);

  if (normalizedValue === '') {
    return {
      normalizedValue,
      mode: 'none',
      query: '',
      owner: '',
      repoPrefix: '',
      isCompleteRepo: false,
    };
  }

  const parts = normalizedValue.split('/');
  if (parts.length > 2) {
    return {
      normalizedValue,
      mode: 'invalid',
      query: '',
      owner: '',
      repoPrefix: '',
      isCompleteRepo: false,
    };
  }

  if (parts.length === 1) {
    return {
      normalizedValue,
      mode: parts[0].length >= 3 ? 'users' : 'none',
      query: parts[0],
      owner: parts[0],
      repoPrefix: '',
      isCompleteRepo: false,
    };
  }

  const [owner, repoPrefix] = parts;
  if (!owner.match(/^[A-Za-z0-9._-]+$/)) {
    return {
      normalizedValue,
      mode: 'invalid',
      query: '',
      owner: '',
      repoPrefix: '',
      isCompleteRepo: false,
    };
  }

  return {
    normalizedValue,
    mode: 'repos',
    query: `${owner}/${repoPrefix}`,
    owner,
    repoPrefix,
    isCompleteRepo: REPO_PATH.test(normalizedValue) && repoPrefix.length >= 2,
  };
}
