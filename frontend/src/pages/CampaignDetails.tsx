import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useWallet } from '@solana/wallet-adapter-react';
import { useWalletModal } from '@solana/wallet-adapter-react-ui';
import bs58 from 'bs58';
import { api } from '../api/client';
import { useAuth } from '../hooks/useAuth';
import { getStateConfig, formatSOL, formatDate } from '../utils/campaign';
import type { Campaign, FinalizePreviewResponse } from '../types';

function AllocationBar({ percentage }: { percentage: number }) {
  const pct = percentage / 100;
  return (
    <div className="w-full bg-solana-dark rounded-full h-2 overflow-hidden">
      <div
        className="h-full rounded-full bg-gradient-to-r from-solana-purple to-solana-green transition-all duration-500"
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}

export default function CampaignDetails() {
  const { id } = useParams<{ id: string }>();
  const { publicKey, signMessage } = useWallet();
  const { setVisible } = useWalletModal();
  const { user } = useAuth();
  const [campaign, setCampaign] = useState<Campaign | null>(null);
  const [preview, setPreview] = useState<FinalizePreviewResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [finalizing, setFinalizing] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [claiming, setClaiming] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [solanaReady, setSolanaReady] = useState(true);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    api
      .getCampaign(id)
      .then((data) => {
        if (!cancelled) setCampaign(data);
      })
      .catch((e) => {
        if (!cancelled) setError(e.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  useEffect(() => {
    let cancelled = false;
    api
      .getHealth()
      .then((health) => {
        if (!cancelled) {
          setSolanaReady(health.solana);
        }
      })
      .catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, []);

  async function handlePreview() {
    if (!id) return;
    setPreviewing(true);
    setError(null);
    try {
      const result = await api.finalizePreview(id);
      setPreview(result);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Preview failed');
    } finally {
      setPreviewing(false);
    }
  }

  async function handleFinalize() {
    if (!id) return;
    setFinalizing(true);
    setError(null);
    try {
      await api.finalize(id);
      const updated = await api.getCampaign(id);
      setCampaign(updated);
      setPreview(null);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Finalization failed');
    } finally {
      setFinalizing(false);
    }
  }

  async function handleClaim(contributor: string) {
    if (!id || !publicKey) {
      if (!publicKey) setError('Connect wallet first');
      return;
    }
    if (!signMessage) {
      setError('This wallet does not support message signing.');
      return;
    }
    if (!solanaReady) {
      setError('Claims are unavailable until the backend is connected to Solana.');
      return;
    }
    setClaiming(contributor);
    setError(null);
    try {
      const challenge = await api.claimChallenge(id, {
        contributor_github: contributor,
        wallet_address: publicKey.toBase58(),
      });
      const signatureBytes = await signMessage(new TextEncoder().encode(challenge.message));
      await api.claimAllocation(
        id,
        contributor,
        publicKey.toBase58(),
        challenge.challenge_id,
        bs58.encode(signatureBytes)
      );
      const updated = await api.getCampaign(id);
      setCampaign(updated);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Claim failed');
    } finally {
      setClaiming(null);
    }
  }

  if (loading) {
    return (
      <div className="text-center py-24">
        <div className="inline-block w-10 h-10 border-2 border-solana-purple border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!campaign) {
    return (
      <div className="card text-center py-16 max-w-lg mx-auto">
        <p className="text-red-400 mb-4">Campaign not found</p>
        <Link to="/" className="btn-secondary text-sm inline-block">
          Back to Campaigns
        </Link>
      </div>
    );
  }

  const isFinalized = campaign.state === 'finalized' || campaign.state === 'completed';
  const isCompleted = campaign.state === 'completed';
  const isPastDeadline = new Date(campaign.deadline) < new Date();
  const isOwner = user?.github_username === campaign.owner_github_username;
  const canShowFinalizeCard = campaign.state === 'funded' && isPastDeadline;
  const stateConfig = getStateConfig(campaign.state, isPastDeadline);

  const badgeClass =
    campaign.state === 'funded'
      ? 'badge-funded'
      : campaign.state === 'finalized'
        ? 'badge-finalized'
        : campaign.state === 'completed'
          ? 'badge-completed'
          : 'badge-created';

  const allocationModeLabel =
    preview?.allocation_mode === 'code_impact'
      ? 'PR diff impact scoring'
      : 'Contributor metrics fallback';
  const previewWindowLabel =
    preview &&
    `${formatDate(preview.snapshot.window_start)} to ${formatDate(preview.snapshot.window_end)}`;

  return (
    <div className="max-w-3xl mx-auto">
      <Link
        to="/"
        className="inline-flex items-center gap-2 text-sm text-gray-500 hover:text-white transition-colors mb-6"
      >
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18" />
        </svg>
        Back to Campaigns
      </Link>

      {/* Header */}
      <div className="card mb-6">
        <div className="flex items-start justify-between mb-6">
          <div className="flex items-start gap-4">
            <div className="w-12 h-12 rounded-xl bg-solana-dark border border-solana-border flex items-center justify-center flex-shrink-0">
              <svg
                className="w-6 h-6 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={1.5}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M3.75 9.776c.112-.017.227-.026.344-.026h15.812c.117 0 .232.009.344.026m-16.5 0a2.25 2.25 0 00-1.883 2.542l.857 6a2.25 2.25 0 002.227 1.932H19.05a2.25 2.25 0 002.227-1.932l.857-6a2.25 2.25 0 00-1.883-2.542m-16.5 0V6A2.25 2.25 0 016 3.75h3.879a1.5 1.5 0 011.06.44l2.122 2.12a1.5 1.5 0 001.06.44H18A2.25 2.25 0 0120.25 9v.776"
                />
              </svg>
            </div>
            <div>
              <h1 className="text-2xl font-bold">{campaign.repo}</h1>
              <p className="text-sm text-gray-500 mt-1 font-mono">{campaign.campaign_id}</p>
              {campaign.owner_github_username && (
                <p className="text-xs text-gray-500 mt-0.5">
                  Created by @{campaign.owner_github_username}
                </p>
              )}
            </div>
          </div>
          <span className={`badge ${badgeClass} flex-shrink-0`}>{stateConfig.label}</span>
        </div>

        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="bg-solana-dark rounded-xl p-3 border border-solana-border/50">
            <span className="text-[10px] text-gray-500 uppercase tracking-wider">Pool</span>
            <p className="text-lg font-bold text-solana-green mt-1">
              {formatSOL(campaign.pool_amount)} SOL
            </p>
          </div>
          <div className="bg-solana-dark rounded-xl p-3 border border-solana-border/50">
            <span className="text-[10px] text-gray-500 uppercase tracking-wider">Deadline</span>
            <p className="text-sm font-semibold mt-1">{formatDate(campaign.deadline)}</p>
          </div>
          <div className="bg-solana-dark rounded-xl p-3 border border-solana-border/50">
            <span className="text-[10px] text-gray-500 uppercase tracking-wider">Created</span>
            <p className="text-sm font-semibold mt-1">{formatDate(campaign.created_at)}</p>
          </div>
          <div className="bg-solana-dark rounded-xl p-3 border border-solana-border/50">
            <span className="text-[10px] text-gray-500 uppercase tracking-wider">Sponsor</span>
            <p className="text-xs font-mono mt-1.5 truncate">
              {campaign.sponsor || campaign.authority || 'N/A'}
            </p>
          </div>
        </div>

        {(isCompleted || isFinalized) && (
          <div className="mt-4 pt-4 border-t border-solana-border grid grid-cols-2 gap-4">
            <div>
              <span className="text-[10px] text-gray-500 uppercase tracking-wider">Claimed</span>
              <p className="text-lg font-semibold mt-1">
                {formatSOL(campaign.total_claimed)} / {formatSOL(campaign.pool_amount)} SOL
              </p>
            </div>
            {campaign.campaign_pda && (
              <div>
                <span className="text-[10px] text-gray-500 uppercase tracking-wider">
                  Campaign PDA
                </span>
                <p className="text-xs font-mono mt-1.5 truncate">{campaign.campaign_pda}</p>
              </div>
            )}
          </div>
        )}

        {campaign.tx_signature && (
          <div className="mt-4 pt-4 border-t border-solana-border">
            <span className="text-[10px] text-gray-500 uppercase tracking-wider">
              Transaction:{' '}
            </span>
            <a
              href={`https://explorer.solana.com/tx/${campaign.tx_signature}?cluster=${import.meta.env.VITE_SOLANA_NETWORK || 'devnet'}`}
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-solana-purple hover:underline font-mono"
            >
              {campaign.tx_signature.slice(0, 20)}...
            </a>
          </div>
        )}
      </div>

      {/* GitHub App Install Banner */}
      {!isFinalized && !isCompleted && (
        <div className="card mb-6 border-blue-500/20 bg-blue-500/[0.03]">
          <div className="flex items-start gap-3">
            <div className="w-10 h-10 rounded-xl bg-blue-500/15 flex items-center justify-center flex-shrink-0">
              <span className="text-lg">🔔</span>
            </div>
            <div className="flex-1">
              <h3 className="text-sm font-semibold text-blue-300">Auto-notify contributors</h3>
              <p className="text-xs text-gray-500 mt-1">
                Install the RepoBounty GitHub App to automatically post reward notifications on
                contributors&apos; PRs after finalization.
              </p>
              <a
                href="https://github.com/apps/repobounty-ai/installations/new"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-block mt-2 text-xs bg-blue-500/15 text-blue-300 hover:bg-blue-500/25 px-3 py-1.5 rounded-full transition-colors border border-blue-500/20"
              >
                Install GitHub App
              </a>
            </div>
          </div>
        </div>
      )}

      {/* Finalize Actions */}
      {canShowFinalizeCard && (
        <div className="card mb-6">
          <h2 className="text-lg font-semibold mb-4">Finalize Campaign</h2>
          <p className="text-sm text-gray-500 mb-4">
            The deadline has passed. Preview now saves a frozen allocation snapshot, and manual
            finalization reuses that exact snapshot for the on-chain finalize call.
          </p>
          {!campaign.owner_github_username ? (
            <p className="text-sm text-yellow-200">
              Manual finalize is unavailable for this legacy campaign because no creator account was
              stored. The backend auto-finalize worker is the remaining safe path.
            </p>
          ) : !user ? (
            <p className="text-sm text-yellow-200">
              Log in as @{campaign.owner_github_username} to run preview or manual finalize.
              Otherwise wait for backend-controlled finalization.
            </p>
          ) : !isOwner ? (
            <p className="text-sm text-yellow-200">
              Only @{campaign.owner_github_username} can run manual preview or finalize for this
              campaign.
            </p>
          ) : (
            <div className="space-y-4">
              {!solanaReady && (
                <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-xl p-4 text-sm text-yellow-200">
                  The backend is not connected to Solana right now. You can still inspect a preview,
                  but on-chain finalization is disabled until the authority is configured again.
                </div>
              )}
              <div className="flex gap-3">
                <button
                  onClick={handlePreview}
                  disabled={previewing}
                  className="btn-secondary text-sm"
                >
                  {previewing ? 'Loading preview...' : 'Preview Allocations'}
                </button>
                {preview && (
                  <button
                    onClick={handleFinalize}
                    disabled={finalizing || !solanaReady}
                    className="btn-primary text-sm"
                  >
                    {finalizing
                      ? 'Finalizing on Solana...'
                      : solanaReady
                        ? 'Finalize on Solana'
                        : 'Solana Backend Required'}
                  </button>
                )}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Preview */}
      {preview && !isFinalized && (
        <div className="card mb-6 border-yellow-500/20">
          <h2 className="text-lg font-semibold mb-1">AI Allocation Preview</h2>
          <p className="text-xs text-gray-500 mb-4">
            Model: {preview.ai_model} | Source: {allocationModeLabel} | Snapshot v
            {preview.snapshot.version}
          </p>
          <p className="text-xs text-gray-600 mb-4">
            Saved at {formatDate(preview.snapshot.created_at)} for contribution window{' '}
            {previewWindowLabel}. Finalize will use this exact snapshot unless campaign inputs
            become stale and you regenerate it.
          </p>
          {preview.snapshot.contributor_notes && (
            <p className="text-xs text-gray-600 mb-4">{preview.snapshot.contributor_notes}</p>
          )}

          {preview.contributors.length > 0 && (
            <div className="mb-4">
              <h3 className="text-sm font-medium text-gray-400 mb-2">Contributor Stats</h3>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="text-left text-gray-500 border-b border-solana-border">
                      <th className="pb-2">User</th>
                      <th className="pb-2">Commits</th>
                      <th className="pb-2">PRs</th>
                      <th className="pb-2">Reviews</th>
                    </tr>
                  </thead>
                  <tbody>
                    {preview.contributors.map((c) => (
                      <tr key={c.username} className="border-b border-solana-border/50">
                        <td className="py-2 font-mono">@{c.username}</td>
                        <td className="py-2">{c.commits}</td>
                        <td className="py-2">{c.pull_requests}</td>
                        <td className="py-2">{c.reviews}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          <h3 className="text-sm font-medium text-gray-400 mb-2">Proposed Allocations</h3>
          <div className="space-y-3">
            {preview.allocations.map((a) => (
              <div key={a.contributor}>
                <div className="flex items-center justify-between text-sm mb-1">
                  <span className="font-mono">@{a.contributor}</span>
                  <span className="font-semibold">
                    {(a.percentage / 100).toFixed(1)}% &middot; {formatSOL(a.amount)} SOL
                  </span>
                </div>
                <AllocationBar percentage={a.percentage} />
                {a.reasoning && <p className="text-xs text-gray-500 mt-1">{a.reasoning}</p>}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Finalized Allocations with Claim Buttons */}
      {isFinalized && campaign.allocations && campaign.allocations.length > 0 && (
        <div className="card">
          <h2 className="text-lg font-semibold mb-4">Final Allocations (On-Chain)</h2>
          <p className="text-xs text-gray-500 mb-4">
            Claims send SOL to the wallet currently connected in Phantom, and that wallet must sign
            a one-time proof before the backend submits the claim.
          </p>
          {!solanaReady && (
            <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-xl p-4 text-sm text-yellow-200 mb-4">
              The backend is not connected to Solana, so claiming is currently disabled.
            </div>
          )}
          <div className="space-y-4">
            {campaign.allocations.map((a) => {
              const isOwnAllocation = user?.github_username === a.contributor;
              const isCurrentlyClaiming = claiming === a.contributor;

              return (
                <div
                  key={a.contributor}
                  className={`p-4 rounded-xl border transition-all ${
                    a.claimed
                      ? 'border-solana-green/30 bg-solana-green/[0.03]'
                      : 'border-solana-border hover:border-solana-purple/30'
                  }`}
                >
                  <div className="flex items-center justify-between mb-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono font-medium">@{a.contributor}</span>
                      {a.claimed && <span className="badge badge-completed text-[10px]">Claimed</span>}
                    </div>
                    <span className="font-bold text-solana-green">
                      {(a.percentage / 100).toFixed(1)}% &middot; {formatSOL(a.amount)} SOL
                    </span>
                  </div>
                  <AllocationBar percentage={a.percentage} />
                  {a.reasoning && <p className="text-xs text-gray-500 mt-1">{a.reasoning}</p>}
                  {isOwnAllocation && !a.claimed && (
                    <div className="mt-3 pt-3 border-t border-solana-border/50">
                      {!publicKey ? (
                        <button
                          onClick={() => setVisible(true)}
                          className="btn-primary text-xs py-1.5 px-4"
                        >
                          Connect Wallet to Claim
                        </button>
                      ) : !solanaReady ? (
                        <button
                          disabled
                          className="btn-primary text-xs py-1.5 px-4 opacity-60 cursor-not-allowed"
                        >
                          Solana Backend Required
                        </button>
                      ) : (
                        <button
                          onClick={() => handleClaim(a.contributor)}
                          disabled={isCurrentlyClaiming}
                          className="btn-primary text-xs py-1.5 px-4"
                        >
                          {isCurrentlyClaiming ? 'Claiming...' : `Claim ${formatSOL(a.amount)} SOL`}
                        </button>
                      )}
                    </div>
                  )}
                  {a.claimed && a.claimant_wallet && (
                    <p className="text-xs text-gray-600 mt-2 font-mono">
                      Claimed by {a.claimant_wallet.slice(0, 8)}...{a.claimant_wallet.slice(-8)}
                    </p>
                  )}
                </div>
              );
            })}
          </div>

          {campaign.finalized_at && (
            <p className="text-xs text-gray-600 mt-6 pt-4 border-t border-solana-border">
              Finalized at {formatDate(campaign.finalized_at)}
            </p>
          )}
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="card border-red-500/30 bg-red-500/[0.05] mt-6">
          <p className="text-sm text-red-400">{error}</p>
        </div>
      )}
    </div>
  );
}
