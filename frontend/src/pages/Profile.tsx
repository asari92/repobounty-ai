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
      <div className="text-center py-20">
        <div className="inline-block w-8 h-8 border-2 border-solana-purple border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!user) {
    return (
      <div className="text-center py-20">
        <p className="text-gray-400">Please log in to view your profile.</p>
      </div>
    );
  }

  const totalAvailable = claims.filter((c) => !c.claimed).reduce((sum, c) => sum + c.amount, 0);
  const loading = Boolean(user) && lastLoadedUser !== user?.github_username;

  return (
    <div className="max-w-2xl mx-auto">
      <h1 className="text-3xl font-bold mb-2">
        <span className="gradient-text">Profile</span>
      </h1>
      <p className="text-gray-400 mb-8">Your account and claimable rewards</p>

      <div className="card mb-6">
        <div className="flex items-center gap-4 mb-4">
          <img
            src={user.avatar_url}
            alt={user.github_username}
            className="w-16 h-16 rounded-full"
          />
          <div>
            <h2 className="text-xl font-bold">@{user.github_username}</h2>
            {user.wallet_address && (
              <p className="text-sm text-gray-400 font-mono">
                Saved profile wallet: {user.wallet_address.slice(0, 6)}...
                {user.wallet_address.slice(-6)}
              </p>
            )}
            {publicKey && (
              <p className="text-sm text-solana-green font-mono">
                Connected claim wallet: {publicKey.toBase58().slice(0, 6)}...
                {publicKey.toBase58().slice(-6)}
              </p>
            )}
          </div>
        </div>
        <p className="text-xs text-gray-500">
          Claims use the wallet currently connected on the campaign page, and that wallet must sign
          a one-time proof before payout. Saved profile wallets remain informational only in this
          MVP.
        </p>
      </div>

      <div className="card">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Claimable Rewards</h2>
          {!loading && (
            <span className="text-sm font-bold text-solana-green">
              {formatSOL(totalAvailable)} SOL available
            </span>
          )}
        </div>

        {error ? (
          <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-4 text-sm text-red-400">
            {error}
          </div>
        ) : loading ? (
          <div className="text-center py-8">
            <div className="inline-block w-8 h-8 border-2 border-solana-purple border-t-transparent rounded-full animate-spin" />
          </div>
        ) : claims.length === 0 ? (
          <p className="text-sm text-gray-400 text-center py-8">
            No claimable rewards found. Finalized campaigns where you contributed will appear here.
          </p>
        ) : (
          <div className="space-y-3">
            {claims.map((claim) => (
              <div
                key={`${claim.campaign_id}-${claim.contributor}`}
                className="p-4 rounded-lg border border-solana-border hover:border-solana-purple/50 transition-colors"
              >
                <div className="flex items-center justify-between mb-1">
                  <div>
                    <span className="font-mono text-sm">{claim.repo}</span>
                    <span className="text-xs text-gray-500 ml-2">
                      #{claim.campaign_id.slice(0, 8)}
                    </span>
                  </div>
                  <span className="font-bold text-solana-green">
                    {claim.amount_sol} SOL ({(claim.percentage / 100).toFixed(1)}%)
                  </span>
                </div>
                <Link
                  to={`/campaign/${claim.campaign_id}`}
                  className="text-xs text-solana-purple hover:underline"
                >
                  View campaign &rarr;
                </Link>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
