import { useEffect, useRef, useState } from 'react';
import { api } from '../api/client';
import { getGitHubSearchState, normalizeGitHubRepoInput } from '../utils/githubRepoInput';
import type { UserSearchResult, RepoSearchResult } from '../types';

interface GitHubAutocompleteProps {
  value: string;
  onChange: (value: string) => void;
}

export default function GitHubAutocomplete({ value, onChange }: GitHubAutocompleteProps) {
  const [results, setResults] = useState<(UserSearchResult | RepoSearchResult)[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const [loading, setLoading] = useState(false);
  const [dropdownMessage, setDropdownMessage] = useState<string | null>(null);
  const [dropdownVisible, setDropdownVisible] = useState(false);
  const requestIdRef = useRef(0);
  const selectedOwnerRef = useRef<string | null>(null);
  const selectedRepoRef = useRef<RepoSearchResult | null>(null);
  const suppressNextSearchRef = useRef(false);

  const searchState = getGitHubSearchState(value);

  const isSelected =
    selectedRepoRef.current !== null &&
    value === `${selectedRepoRef.current.owner}/${selectedRepoRef.current.name}`;

  useEffect(() => {
    if (isSelected) {
      setResults([]);
      setSelectedIndex(-1);
      setLoading(false);
      setDropdownMessage(null);
      setDropdownVisible(false);
      return;
    }

    if (suppressNextSearchRef.current) {
      suppressNextSearchRef.current = false;
      setResults([]);
      setSelectedIndex(-1);
      setLoading(false);
      setDropdownMessage(null);
      setDropdownVisible(false);
      return;
    }

    if (searchState.mode !== 'users' && searchState.mode !== 'repos') {
      setResults([]);
      setSelectedIndex(-1);
      setLoading(false);
      setDropdownMessage(null);
      setDropdownVisible(false);
      return;
    }

    const requestId = ++requestIdRef.current;
    setLoading(true);
    setDropdownMessage(null);
    setDropdownVisible(true);

    const timer = setTimeout(async () => {
      try {
        const searchResults = await api.searchGitHub(searchState.query);
        if (requestIdRef.current !== requestId) return;

        const items = Array.isArray(searchResults) ? searchResults : [];
        setResults(items);
        setSelectedIndex(-1);

        if (searchState.isCompleteRepo && selectedRepoRef.current === null) {
          const match = items.find(
            (r): r is RepoSearchResult =>
              'owner' in r && r.owner === searchState.owner && r.name === searchState.repoPrefix
          );
          if (match) {
            selectedOwnerRef.current = match.owner;
            selectedRepoRef.current = match;
            setResults([]);
            setDropdownMessage(null);
            setDropdownVisible(false);
            setLoading(false);
            return;
          }
        }

        if (items.length === 0) {
          setDropdownMessage(
            searchState.mode === 'repos' && searchState.repoPrefix === ''
              ? 'No public repositories found'
              : searchState.mode === 'repos'
                ? 'No matching repositories'
                : 'No matching GitHub users'
          );
        } else {
          setDropdownMessage(null);
        }
      } catch {
        if (requestIdRef.current !== requestId) return;
        setResults([]);
        setSelectedIndex(-1);
        setDropdownMessage('Could not load GitHub results');
      } finally {
        if (requestIdRef.current === requestId) {
          setLoading(false);
        }
      }
    }, 300);

    return () => clearTimeout(timer);
  }, [searchState.mode, searchState.query, searchState.repoPrefix, isSelected]);

  function handleInputChange(e: React.ChangeEvent<HTMLInputElement>) {
    const raw = normalizeGitHubRepoInput(e.target.value);
    const nextState = getGitHubSearchState(raw);

    if (selectedOwnerRef.current) {
      if (
        nextState.mode === 'none' ||
        nextState.mode === 'users' ||
        (nextState.mode === 'repos' && nextState.owner !== selectedOwnerRef.current)
      ) {
        selectedOwnerRef.current = null;
        selectedRepoRef.current = null;
      }
    }

    if (
      selectedRepoRef.current &&
      raw !== `${selectedRepoRef.current.owner}/${selectedRepoRef.current.name}`
    ) {
      selectedRepoRef.current = null;
    }

    suppressNextSearchRef.current = false;
    onChange(raw);
    setSelectedIndex(-1);
  }

  function handleSelectResult(result: UserSearchResult | RepoSearchResult) {
    if ('login' in result) {
      selectedOwnerRef.current = result.login;
      selectedRepoRef.current = null;
      suppressNextSearchRef.current = false;
      onChange(`${result.login}/`);
    } else {
      selectedOwnerRef.current = result.owner;
      selectedRepoRef.current = result;
      suppressNextSearchRef.current = true;
      onChange(`${result.owner}/${result.name}`);
    }

    setResults([]);
    setSelectedIndex(-1);
    setDropdownMessage(null);
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (!dropdownVisible || results.length === 0) return;

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex((prev) => Math.min(prev + 1, results.length - 1));
        break;
      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex((prev) => Math.max(prev - 1, 0));
        break;
      case 'Enter':
        e.preventDefault();
        if (selectedIndex >= 0) {
          handleSelectResult(results[selectedIndex]);
        }
        break;
      case 'Escape':
        e.preventDefault();
        setResults([]);
        setDropdownMessage(null);
        setDropdownVisible(false);
        break;
    }
  }

  return (
    <div className="relative">
      <input
        type="text"
        value={value}
        onChange={handleInputChange}
        onKeyDown={handleKeyDown}
        placeholder="owner/repo"
        className="input"
        autoFocus={false}
      />
      {dropdownVisible && (
        <ul
          role="listbox"
          className="absolute z-10 w-full mt-1 bg-solana-card border border-solana-border rounded-lg shadow-lg max-h-60 overflow-y-auto"
        >
          {loading && (
            <li className="px-4 py-2 text-sm text-gray-400" aria-disabled="true">
              Loading GitHub results...
            </li>
          )}

          {!loading && dropdownMessage && (
            <li className="px-4 py-2 text-sm text-gray-400" aria-disabled="true">
              {dropdownMessage}
            </li>
          )}

          {!loading &&
            results.map((result, index) => (
              <li
                key={'login' in result ? result.login : `${result.owner}/${result.name}`}
                role="option"
                aria-selected={selectedIndex === index}
                aria-label={
                  'login' in result
                    ? result.login
                    : `${result.owner}/${result.name}`
                }
                className={`px-4 py-2 cursor-pointer hover:bg-solana-green/10 flex items-center gap-3 ${
                  selectedIndex === index ? 'bg-solana-green/20' : ''
                }`}
                onClick={() => handleSelectResult(result)}
              >
                {'login' in result ? (
                  <>
                    <img
                      src={result.avatar_url}
                      alt={result.login}
                      className="w-6 h-6 rounded-full"
                    />
                    <span className="text-gray-300">{result.login}</span>
                  </>
                ) : (
                  <span className="text-gray-300">
                    {result.owner}/{result.name}
                  </span>
                )}
              </li>
            ))}
        </ul>
      )}
    </div>
  );
}
