import { useState, useEffect } from 'react';
import { api } from '../api/client';
import type { UserSearchResult, RepoSearchResult } from '../types';

interface GitHubAutocompleteProps {
  value: string;
  onChange: (value: string) => void;
}

export default function GitHubAutocomplete({ value, onChange }: GitHubAutocompleteProps) {
  const [results, setResults] = useState<(UserSearchResult | RepoSearchResult)[]>([]);
  const [selectedIndex, setSelectedIndex] = useState(-1);

  const showDropdown = value.length >= 2 && results.length > 0;

  useEffect(() => {
    if (value.length < 2) {
      setResults([]); // eslint-disable-line react-hooks/set-state-in-effect
      return;
    }

    const timer = setTimeout(async () => {
      try {
        const searchResults = await api.searchGitHub(value);
        setResults(searchResults);
      } catch (err) {
        console.error('Failed to search GitHub:', err);
        setResults([]);
      }
    }, 300);

    return () => {
      clearTimeout(timer);
    };
  }, [value]);

  function handleInputChange(e: React.ChangeEvent<HTMLInputElement>) {
    onChange(e.target.value);
    setSelectedIndex(-1);
  }

  function handleSelectResult(result: UserSearchResult | RepoSearchResult) {
    const login = 'login' in result ? result.login : result.owner;
    const name = 'name' in result ? result.name : result.login;
    const newValue = value.endsWith('/') ? `${value}${name}` : `${login}/`;
    onChange(newValue);
    setResults([]);
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
      {showDropdown && results.length > 0 && (
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
  );
}
