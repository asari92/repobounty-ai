import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useWallet } from '@solana/wallet-adapter-react';
import { useAuth } from '../hooks/useAuth';
import { api } from '../api/client';
import { formatSOL } from '../utils/campaign';
import type { ClaimItem, MyCampaign, User } from '../types';
import type { PublicKey } from '@solana/web3.js';

export default function Profile() {
  const { publicKey } = useWallet();
  const { user } = useAuth();
  const [claims, setClaims] = useState<ClaimItem[]>([]);
  const [myCampaigns, setMyCampaigns] = useState<MyCampaign[]>([]);
  const [errorClaims, setErrorClaims] = useState<string | null>(null);
  const [errorCampaigns, setErrorCampaigns] = useState<string | null>(null);

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
          setErrorClaims(null);
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setClaims([]);
          setErrorClaims(e instanceof Error ? e.message : 'Failed to load claims');
        }
      });
    return () => {
      cancelled = true;
    };
  }, [user]);

  useEffect(() => {
    const walletAddr = publicKey?.toBase58();
    if (!walletAddr) {
      return;
    }
    let cancelled = false;
    api
      .getMyCampaigns(walletAddr)
      .then((data) => {
        if (!cancelled) {
          setMyCampaigns(data);
          setErrorCampaigns(null);
        }
      })
      .catch((e) => {
        if (!cancelled) {
          setMyCampaigns([]);
          setErrorCampaigns(e instanceof Error ? e.message : 'Failed to load campaigns');
        }
      });
    return () => {
      cancelled = true;
    };
  }, [publicKey]);

  if (!user && !publicKey) {
    return (
      <div className="text-center py-24">
        <p className="text-gray-500 text-sm">Connect GitHub or wallet to view your profile.</p>
      </div>
    );
  }

  const walletAddr = publicKey?.toBase58() || user?.wallet_address;
  const totalAllocated = claims.reduce((sum, c) => sum + c.amount, 0);
  const totalAvailable = claims.filter((c) => !c.claimed).reduce((sum, c) => sum + c.amount, 0);
  const totalClaimed = claims.filter((c) => c.claimed).reduce((sum, c) => sum + c.amount, 0);
  const totalFunded = myCampaigns.reduce((sum, c) => sum + c.pool_amount, 0);

  return (
    <div className="max-w-5xl mx-auto">
      {/* User header */}
      <div className="card card-accent mb-5 animate-fade-in-up">
        <div className="flex items-center gap-4">
          {user ? (
            <img
              src={user.avatar_url}
              alt={user.github_username}
              className="w-14 h-14 rounded-full ring-2 ring-solana-border flex-shrink-0"
            />
          ) : (
            <div className="w-14 h-14 rounded-full bg-solana-purple/20 flex-shrink-0 flex items-center justify-center">
              <span className="text-2xl">💼</span>
            </div>
          )}
          <div className="flex-1 min-w-0">
            {user ? (
              <h1 className="text-lg font-bold">@{user.github_username}</h1>
            ) : (
              <h1 className="text-lg font-bold">Wallet Connected</h1>
            )}
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

      {/* Contributor Rewards */}
      {user && (
        <div className="mb-5 animate-fade-in-up" style={{ animationDelay: '80ms' }}>
          <h2 className="text-sm font-semibold text-gray-300 mb-3">
            Contributor Rewards
          </h2>

          <div className="grid grid-cols-3 gap-3 mb-3">
            <div className="stat-block text-center">
              <p className="text-[10px] text-gray-600 uppercase tracking-wider mb-0.5">
                Available
              </p>
              <p className="text-xl font-bold text-solana-green">{formatSOL(totalAvailable)}</p>
            </div>
            <div className="stat-block text-center">
              <p className="text-[10px] text-gray-600 uppercase tracking-wider mb-0.5">
                Claimed
              </p>
              <p className="text-xl font-bold">{formatSOL(totalClaimed)}</p>
            </div>
            <div className="stat-block text-center">
              <p className="text-[10px] text-gray-600 uppercase tracking-wider mb-0.5">
                Total Allocated
              </p>
              <p className="text-xl font-bold">{formatSOL(totalAllocated)}</p>
            </div>
          </div>

          <div className="card">
            <div className="flex items-center justify-between mb-4">
              <span className="text-xs text-gray-500">
                {claims.length} reward{claims.length !== 1 ? 's' : ''} from {new Set(claims.map((c) => c.campaign_id)).size} campaign{new Set(claims.map((c) => c.campaign_id)).size !== 1 ? 's' : ''}
              </span>
              <span className="text-sm font-bold text-solana-green">
                {formatSOL(totalAvailable)} SOL available
              </span>
            </div>

            {errorClaims ? (
              <div className="bg-red-500/5 border border-red-500/15 rounded-lg p-3 text-xs text-red-400">
                {errorClaims}
              </div>
            ) : claims.length === 0 ? (
              <p className="text-xs text-gray-600 text-center py-4">
                No rewards yet. Contribute to finalized campaigns to earn rewards.
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
                        <span className="text-xs font-mono text-gray-300 truncate">
                          {claim.repo}
                        </span>
                        {claim.claimed ? (
                          <span className="badge badge-completed text-[9px]">Claimed</span>
                        ) : (
                          <span className="badge badge-funded text-[9px]">Available</span>
                        )}
                      </div>
                      <div className="flex items-center gap-3 flex-shrink-0">
                        <span className="text-xs text-gray-500">
                          {(claim.percentage / 100).toFixed(1)}%
                        </span>
                        <span
                          className={`text-sm font-semibold ${claim.claimed ? 'text-gray-400' : 'text-solana-green'}`}
                        >
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
        </div>
      )}

      {/* Sponsored Campaigns */}
      {publicKey && (
        <div className="mb-5 animate-fade-in-up" style={{ animationDelay: '160ms' }}>
          <h2 className="text-sm font-semibold text-gray-300 mb-3">Sponsored Campaigns</h2>

          <div className="grid grid-cols-3 gap-3 mb-3">
            <div className="stat-block text-center">
              <p className="text-[10px] text-gray-600 uppercase">Campaigns</p>
              <p className="text-xl font-bold">{myCampaigns.length}</p>
            </div>
            <div className="stat-block text-center">
              <p className="text-[10px] text-gray-600 uppercase">Active</p>
              <p className="text-xl font-bold">
                {myCampaigns.filter((c) => c.state === 'active' || c.state === 'funded').length}
              </p>
            </div>
            <div className="stat-block text-center">
              <p className="text-[10px] text-gray-600 uppercase">Total Funded</p>
              <p className="text-xl font-bold text-solana-green">{formatSOL(totalFunded)}</p>
            </div>
          </div>

          <div className="card">
            {errorCampaigns && (
              <div className="bg-red-500/5 border border-red-500/15 rounded-lg p-3 text-xs text-red-400 mb-4">
                {errorCampaigns}
              </div>
            )}

            {myCampaigns.length === 0 ? (
              <p className="text-xs text-gray-600 text-center py-4">
                No sponsored campaigns yet for this wallet.
              </p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b border-solana-border/50">
                      <th className="text-left py-2 px-3 text-gray-500">Campaign</th>
                      <th className="text-left py-2 px-3 text-gray-500">Repo</th>
                      <th className="text-left py-2 px-3 text-gray-500">State</th>
                      <th className="text-left py-2 px-3 text-gray-500">Amount</th>
                      <th className="text-left py-2 px-3 text-gray-500">Deadline</th>
                      <th className="text-right py-2 px-3 text-gray-500">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {myCampaigns.map((c) => (
                      <SponsoredCampaignRow
                        key={c.campaign_id}
                        campaign={c}
                        publicKey={publicKey}
                        user={user}
                      />
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Quick actions */}
      <div
        className="grid grid-cols-2 gap-3 animate-fade-in-up"
        style={{ animationDelay: '260ms' }}
      >
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

function SponsoredCampaignRow({
  campaign,
  publicKey,
  user,
}: {
  campaign: MyCampaign;
  publicKey: PublicKey | null;
  user: User | null;
}) {
  const navigate = useNavigate();
  const walletAddr = publicKey?.toBase58() || user?.wallet_address;
  const canRefund = walletAddr && campaign.can_refund;

  const stateColors: Record<string, string> = {
    active: 'text-solana-green',
    funded: 'text-solana-green',
    finalized: 'text-solana-purple',
    closed: 'text-gray-500',
    completed: 'text-solana-purple',
  };

  return (
    <tr className="border-b border-solana-border/30 hover:bg-solana-card-hover/50 transition-colors">
      <td className="py-2.5 px-3 font-mono text-gray-300">
        <Link
          to={`/campaign/${campaign.campaign_id}`}
          className="hover:text-solana-purple transition-colors"
        >
          {campaign.campaign_id}
        </Link>
      </td>
      <td className="py-2.5 px-3">
        <Link
          to={`/campaign/${campaign.campaign_id}`}
          className="hover:text-solana-purple transition-colors"
        >
          {campaign.repo}
        </Link>
      </td>
      <td className="py-2.5 px-3">
        <span className={`capitalize ${stateColors[campaign.state] || 'text-gray-400'}`}>
          {campaign.state}
        </span>
      </td>
      <td className="py-2.5 px-3 font-semibold text-solana-green">
        {formatSOL(campaign.pool_amount)} SOL
      </td>
      <td className="py-2.5 px-3 text-gray-400">
        {new Date(campaign.deadline).toLocaleDateString()}
      </td>
      <td className="py-2.5 px-3 text-right">
        <div className="flex items-center justify-end gap-1.5">
          {canRefund && (
            <button
              onClick={() => navigate(`/campaign/${campaign.campaign_id}?action=refund`)}
              className="btn-secondary text-[10px] px-2 py-1"
            >
              Refund
            </button>
          )}
        </div>
      </td>
    </tr>
  );
}
