import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useWallet } from '@solana/wallet-adapter-react';
import { useAuth } from '../hooks/useAuth';
import { api } from '../api/client';
import { formatSOL } from '../utils/campaign';
import type { ClaimItem } from '../types';

function WalletBadge({ address }: { address: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 bg-solana-dark border border-solana-border rounded-full px-3 py-1 text-xs font-mono text-gray-400">
      <span className="w-2 h-2 rounded-full bg-solana-purple" />
      {address.slice(0, 4)}...{address.slice(-4)}
      <button
        onClick={() => navigator.clipboard.writeText(address)}
        className="text-gray-500 hover:text-white transition-colors ml-1"
        title="Copy address"
      >
        <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9.75a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184"
          />
        </svg>
      </button>
    </span>
  );
}

export default function Profile() {
  const { publicKey } = useWallet();
  const { user, isLoading } = useAuth();
  const [claims, setClaims] = useState<ClaimItem[]>([]);
  const [lastLoadedUser, setLastLoadedUser] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!user) {
      return;
    }
    let cancelled = false;
    api
      .getClaims()
      .then((data) => {
        if (!cancelled) {
          setClaims(data);
          setError(null);
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setClaims([]);
          setError(e instanceof Error ? e.message : 'Failed to load claims');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLastLoadedUser(user.github_username);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [user]);

  if (isLoading) {
    return (
      <div className="text-center py-24">
        <div className="inline-block w-10 h-10 border-2 border-solana-purple border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!user) {
    return (
      <div className="text-center py-24">
        <div className="w-20 h-20 rounded-full bg-solana-card border border-solana-border flex items-center justify-center mx-auto mb-4">
          <svg
            className="w-10 h-10 text-gray-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={1.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z"
            />
          </svg>
        </div>
        <p className="text-gray-400">Please log in to view your profile.</p>
      </div>
    );
  }

  const totalAvailable = claims.filter((c) => !c.claimed).reduce((sum, c) => sum + c.amount, 0);
  const totalClaimed = claims.filter((c) => c.claimed).reduce((sum, c) => sum + c.amount, 0);
  const loading = Boolean(user) && lastLoadedUser !== user?.github_username;
  const claimedCount = claims.filter((c) => c.claimed).length;

  return (
    <div className="max-w-3xl mx-auto">
      {/* User card */}
      <div className="card text-center mb-6">
        <div className="flex flex-col items-center py-4">
          <img
            src={user.avatar_url}
            alt={user.github_username}
            className="w-24 h-24 rounded-full ring-4 ring-solana-border mb-4"
          />
          <h1 className="text-2xl font-bold mb-2">@{user.github_username}</h1>
          {(publicKey || user.wallet_address) && (
            <WalletBadge address={publicKey ? publicKey.toBase58() : user.wallet_address!} />
          )}
        </div>

        {/* Stats row */}
        <div className="grid grid-cols-3 gap-4 mt-6 pt-6 border-t border-solana-border">
          <div className="text-center">
            <p className="text-xs text-gray-500 uppercase tracking-wider mb-1">Bounties</p>
            <p className="text-2xl font-bold">{claims.length}</p>
          </div>
          <div className="text-center">
            <p className="text-xs text-gray-500 uppercase tracking-wider mb-1">Claimed</p>
            <p className="text-2xl font-bold">{claimedCount}</p>
          </div>
          <div className="text-center">
            <p className="text-xs text-gray-500 uppercase tracking-wider mb-1">Earned</p>
            <p className="text-2xl font-bold text-solana-green">
              {formatSOL(totalClaimed)}
            </p>
          </div>
        </div>
      </div>

      {/* Rewards Portfolio */}
      <div className="card mb-6">
        <h2 className="text-lg font-semibold mb-6">Rewards Portfolio</h2>

        <div className="text-center py-6 mb-6 rounded-xl bg-solana-dark border border-solana-border">
          <div className="w-14 h-14 rounded-full bg-gradient-to-br from-solana-purple to-solana-green mx-auto mb-3 flex items-center justify-center shadow-lg shadow-solana-purple/20">
            <svg className="w-7 h-7 text-white" viewBox="0 0 24 24" fill="none">
              <circle cx="12" cy="12" r="8" stroke="currentColor" strokeWidth="1.5" />
              <text
                x="12"
                y="16"
                textAnchor="middle"
                fill="currentColor"
                fontSize="10"
                fontWeight="bold"
                fontFamily="monospace"
              >
                S
              </text>
            </svg>
          </div>
          <p className="text-3xl font-bold text-solana-green mb-1">
            {formatSOL(totalAvailable)} SOL
          </p>
          <p className="text-sm text-gray-500">available to claim</p>
        </div>

        {error ? (
          <div className="bg-red-500/10 border border-red-500/30 rounded-xl p-4 text-sm text-red-400">
            {error}
          </div>
        ) : loading ? (
          <div className="text-center py-8">
            <div className="inline-block w-8 h-8 border-2 border-solana-purple border-t-transparent rounded-full animate-spin" />
          </div>
        ) : claims.length === 0 ? (
          <p className="text-sm text-gray-500 text-center py-6">
            No claimable rewards found. Finalized campaigns where you contributed will appear here.
          </p>
        ) : (
          <div className="space-y-3">
            {claims.map((claim) => (
              <Link
                key={`${claim.campaign_id}-${claim.contributor}`}
                to={`/campaign/${claim.campaign_id}`}
                className="block"
              >
                <div className="p-4 rounded-xl border border-solana-border hover:border-solana-purple/40 transition-all hover:bg-solana-card-hover group">
                  <div className="flex items-center justify-between mb-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm text-white">{claim.repo}</span>
                      <span className="text-xs text-gray-600">
                        #{claim.campaign_id.slice(0, 8)}
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="font-bold text-solana-green">
                        {claim.amount_sol} SOL
                      </span>
                      {claim.claimed && (
                        <span className="badge badge-completed text-[10px]">Claimed</span>
                      )}
                    </div>
                  </div>
                  <div className="flex items-center justify-between mt-1">
                    <span className="text-xs text-gray-500">
                      {(claim.percentage / 100).toFixed(1)}% allocation
                    </span>
                    <span className="text-xs text-solana-purple opacity-0 group-hover:opacity-100 transition-opacity">
                      View campaign &rarr;
                    </span>
                  </div>
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>

      {/* Action cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <Link to="/" className="card card-hover group cursor-pointer">
          <div className="flex items-start justify-between">
            <div className="w-10 h-10 rounded-xl bg-solana-purple/15 flex items-center justify-center mb-3">
              <svg
                className="w-5 h-5 text-solana-purple"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={1.5}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 010 3.75H5.625a1.875 1.875 0 010-3.75z"
                />
              </svg>
            </div>
            <svg
              className="w-4 h-4 text-gray-600 group-hover:text-solana-purple transition-colors"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25"
              />
            </svg>
          </div>
          <h3 className="font-semibold text-white mb-1">Activity Log</h3>
          <p className="text-xs text-gray-500">
            Review your contribution history across campaigns.
          </p>
        </Link>

        <Link to="/create" className="card card-hover group cursor-pointer">
          <div className="flex items-start justify-between">
            <div className="w-10 h-10 rounded-xl bg-solana-green/15 flex items-center justify-center mb-3">
              <svg
                className="w-5 h-5 text-solana-green"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={1.5}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M12 4.5v15m7.5-7.5h-15"
                />
              </svg>
            </div>
            <svg
              className="w-4 h-4 text-gray-600 group-hover:text-solana-green transition-colors"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25"
              />
            </svg>
          </div>
          <h3 className="font-semibold text-white mb-1">New Campaign</h3>
          <p className="text-xs text-gray-500">
            Create a new bounty campaign and fund contributors.
          </p>
        </Link>
      </div>
    </div>
  );
}
