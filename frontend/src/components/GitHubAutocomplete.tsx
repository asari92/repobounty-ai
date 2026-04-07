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
  const requestIdRef = useRef(0);

  const searchState = getGitHubSearchState(value);
  const showDropdown =
    searchState.mode === 'users' ||
    searchState.mode === 'repos'
      ? loading || dropdownMessage !== null || results.length > 0
      : false;

  useEffect(() => {
    if (searchState.mode !== 'users' && searchState.mode !== 'repos') {
      setResults([]);
      setSelectedIndex(-1);
      setLoading(false);
      setDropdownMessage(null);
      return;
    }

    const requestId = ++requestIdRef.current;
    setLoading(true);
    setDropdownMessage(null);

    const timer = setTimeout(async () => {
      try {
        const searchResults = await api.searchGitHub(searchState.query);
        if (requestIdRef.current !== requestId) return;

        setResults(Array.isArray(searchResults) ? searchResults : []);
        setSelectedIndex(-1);

        if (searchResults.length === 0) {
          setDropdownMessage(
            searchState.mode === 'repos' && searchState.repoPrefix === ''
              ? 'No public repositories found'
              : searchState.mode === 'repos'
                ? 'No matching repositories'
                : 'No matching GitHub users'
          );
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
  }, [searchState.mode, searchState.query, searchState.repoPrefix]);

  function handleInputChange(e: React.ChangeEvent<HTMLInputElement>) {
    onChange(normalizeGitHubRepoInput(e.target.value));
    setSelectedIndex(-1);
  }

  function handleSelectResult(result: UserSearchResult | RepoSearchResult) {
    if ('login' in result) {
      onChange(`${result.login}/`);
    } else {
      onChange(`${result.owner}/${result.name}`);
    }

    setResults([]);
    setSelectedIndex(-1);
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (!showDropdown || results.length === 0) return;

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
      {showDropdown && (
        <div className="absolute z-10 w-full mt-1 bg-solana-card border border-solana-border rounded-lg shadow-lg max-h-60 overflow-y-auto">
          {loading && (
            <div className="px-4 py-2 text-sm text-gray-400">
              Loading GitHub results...
            </div>
          )}

          {!loading && dropdownMessage && (
            <div className="px-4 py-2 text-sm text-gray-400">{dropdownMessage}</div>
          )}

          {!loading && results.length > 0 && (
            <ul
              role="listbox"
              className="absolute z-10 w-full mt-1 bg-solana-card border border-solana-border rounded-lg shadow-lg max-h-60 overflow-y-auto"
            >
              {results.map((result, index) => (
                <li
                  key={'login' in result ? result.login : `${result.owner}/${result.name}`}
                  role="option"
                  aria-selected={selectedIndex === index}
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
      )}
    </div>
  );
}
