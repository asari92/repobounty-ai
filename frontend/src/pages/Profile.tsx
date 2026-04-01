import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useWallet } from '@solana/wallet-adapter-react';
import { useAuth } from '../hooks/useAuth';
import { api } from '../api/client';
import { formatSOL } from '../utils/campaign';
import type { ClaimItem } from '../types';

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
        <div className="inline-block w-8 h-8 border-2 border-solana-purple border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!user) {
    return (
      <div className="text-center py-24">
        <p className="text-gray-500 text-sm">Please log in to view your profile.</p>
      </div>
    );
  }

  const walletAddr = publicKey ? publicKey.toBase58() : user.wallet_address;
  const totalAvailable = claims.filter((c) => !c.claimed).reduce((sum, c) => sum + c.amount, 0);
  const totalClaimed = claims.filter((c) => c.claimed).reduce((sum, c) => sum + c.amount, 0);
  const loading = Boolean(user) && lastLoadedUser !== user?.github_username;
  const claimedCount = claims.filter((c) => c.claimed).length;

  return (
    <div className="max-w-2xl mx-auto">
      {/* User header — horizontal */}
      <div className="card card-accent mb-5 animate-fade-in-up">
        <div className="flex items-center gap-4">
          <img
            src={user.avatar_url}
            alt={user.github_username}
            className="w-14 h-14 rounded-full ring-2 ring-solana-border flex-shrink-0"
          />
          <div className="flex-1 min-w-0">
            <h1 className="text-lg font-bold">@{user.github_username}</h1>
            {walletAddr && (
              <button
                onClick={() => navigator.clipboard.writeText(walletAddr)}
                className="text-xs font-mono text-gray-500 hover:text-gray-300 transition-colors mt-0.5 truncate block max-w-full"
                title="Copy address"
              >
                {walletAddr.slice(0, 6)}...{walletAddr.slice(-4)} &#x2398;
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Stats inline */}
      <div className="grid grid-cols-3 gap-3 mb-5 animate-fade-in-up" style={{ animationDelay: '80ms' }}>
        <div className="stat-block text-center">
          <p className="text-[10px] text-gray-600 uppercase tracking-wider mb-0.5">Bounties</p>
          <p className="text-xl font-bold">{claims.length}</p>
        </div>
        <div className="stat-block text-center">
          <p className="text-[10px] text-gray-600 uppercase tracking-wider mb-0.5">Claimed</p>
          <p className="text-xl font-bold">{claimedCount}</p>
        </div>
        <div className="stat-block text-center">
          <p className="text-[10px] text-gray-600 uppercase tracking-wider mb-0.5">Earned</p>
          <p className="text-xl font-bold text-solana-green">{formatSOL(totalClaimed)}</p>
        </div>
      </div>

      {/* Available balance */}
      <div className="card mb-5 animate-fade-in-up" style={{ animationDelay: '140ms' }}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-sm font-semibold text-gray-300">Available Balance</h2>
          <span className="text-2xl font-bold text-solana-green">{formatSOL(totalAvailable)} SOL</span>
        </div>

        {error ? (
          <div className="bg-red-500/5 border border-red-500/15 rounded-lg p-3 text-xs text-red-400">
            {error}
          </div>
        ) : loading ? (
          <div className="text-center py-6">
            <div className="inline-block w-6 h-6 border-2 border-solana-purple border-t-transparent rounded-full animate-spin" />
          </div>
        ) : claims.length === 0 ? (
          <p className="text-xs text-gray-600 text-center py-4">
            No claimable rewards. Contribute to finalized campaigns to earn rewards.
          </p>
        ) : (
          <div className="space-y-2">
            {claims.map((claim) => (
              <Link
                key={`${claim.campaign_id}-${claim.contributor}`}
                to={`/campaign/${claim.campaign_id}`}
                className="block group"
              >
                <div className="flex items-center justify-between px-3 py-2.5 rounded-lg border border-solana-border/50 hover:border-solana-border-light hover:bg-solana-card-hover transition-all duration-200">
                  <div className="flex items-center gap-3 min-w-0">
                    <span className="text-xs font-mono text-gray-300 truncate">{claim.repo}</span>
                    {claim.claimed && (
                      <span className="badge badge-completed text-[9px]">Claimed</span>
                    )}
                  </div>
                  <div className="flex items-center gap-3 flex-shrink-0">
                    <span className="text-xs text-gray-500">
                      {(claim.percentage / 100).toFixed(1)}%
                    </span>
                    <span className="text-sm font-semibold text-solana-green">
                      {claim.amount_sol} SOL
                    </span>
                    <span className="text-[10px] text-gray-600 group-hover:text-solana-purple transition-colors">
                      &rarr;
                    </span>
                  </div>
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>

      {/* Quick actions */}
      <div className="grid grid-cols-2 gap-3 animate-fade-in-up" style={{ animationDelay: '200ms' }}>
        <Link
          to="/"
          className="stat-block group hover:border-solana-purple/30 transition-all duration-200 cursor-pointer"
        >
          <p className="text-xs font-semibold text-white mb-0.5">Browse Campaigns</p>
          <p className="text-[10px] text-gray-600 group-hover:text-gray-400 transition-colors">
            View all active bounties
          </p>
        </Link>
        <Link
          to="/create"
          className="stat-block group hover:border-solana-green/30 transition-all duration-200 cursor-pointer"
        >
          <p className="text-xs font-semibold text-white mb-0.5">New Campaign</p>
          <p className="text-[10px] text-gray-600 group-hover:text-gray-400 transition-colors">
            Fund open-source contributors
          </p>
        </Link>
      </div>
    </div>
  );
}
