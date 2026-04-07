import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useConnection, useWallet } from '@solana/wallet-adapter-react';
import { useWalletModal } from '@solana/wallet-adapter-react-ui';
import { Transaction } from '@solana/web3.js';
import bs58 from 'bs58';
import { api } from '../api/client';
import { useAuth } from '../hooks/useAuth';
import { getStateConfig, formatSOL, formatDate } from '../utils/campaign';
import type { Campaign, FinalizePreviewResponse } from '../types';

function AllocationBar({ percentage }: { percentage: number }) {
  const pct = percentage / 100;
  return (
    <div className="w-full bg-solana-dark rounded-sm h-1.5 overflow-hidden">
      <div
        className="h-full rounded-sm bg-gradient-to-r from-solana-purple to-solana-green transition-all duration-500"
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}

export default function CampaignDetails() {
  const { id } = useParams<{ id: string }>();
  const { connection } = useConnection();
  const { publicKey, signMessage, signTransaction } = useWallet();
  const { setVisible } = useWalletModal();
  const { user } = useAuth();
  const [campaign, setCampaign] = useState<Campaign | null>(null);
  const [preview, setPreview] = useState<FinalizePreviewResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [finalizing, setFinalizing] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [claiming, setClaiming] = useState<string | null>(null);
  const [sponsorFinalizing, setSponsorFinalizing] = useState(false);
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

  async function handleSponsorFinalize() {
    if (!id || !publicKey || !signMessage) return;
    setSponsorFinalizing(true);
    setError(null);
    try {
      const challenge = await api.finalizeChallenge(id, publicKey.toBase58());
      const signatureBytes = await signMessage(new TextEncoder().encode(challenge.message));
      await api.finalizeWallet(
        id,
        publicKey.toBase58(),
        challenge.challenge_id,
        bs58.encode(signatureBytes)
      );
      const updated = await api.getCampaign(id);
      setCampaign(updated);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Finalization failed');
    } finally {
      setSponsorFinalizing(false);
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
    if (!signTransaction) {
      setError('This wallet does not support transaction signing.');
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
      const claimTx = await api.claimAllocation(
        id,
        contributor,
        publicKey.toBase58(),
        challenge.challenge_id,
        bs58.encode(signatureBytes)
      );
      const txBytes = bs58.decode(claimTx.partial_tx);
      const transaction = Transaction.from(txBytes);
      const signedTransaction = await signTransaction(transaction);
      const txSignature = await connection.sendRawTransaction(signedTransaction.serialize());
      await connection.confirmTransaction(txSignature, 'confirmed');
      await api.claimConfirm(id, contributor, publicKey.toBase58(), txSignature);
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
        <div className="inline-block w-8 h-8 border-2 border-solana-purple border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!campaign) {
    return (
      <div className="card text-center py-12 max-w-md mx-auto">
        <p className="text-red-400 text-sm mb-3">Campaign not found</p>
        <Link to="/" className="btn-secondary text-xs inline-block">
          ← Back
        </Link>
      </div>
    );
  }

  const isFinalized = campaign.state === 'finalized' || campaign.state === 'completed';
  const isCompleted = campaign.state === 'completed';
  const isPastDeadline = new Date(campaign.deadline) < new Date();
  const isOwner = user?.github_username === campaign.owner_github_username;
  const isSponsor = !!publicKey && publicKey.toBase58() === campaign.sponsor;
  const canSponsorFinalize = campaign.state === 'funded' && isPastDeadline && isSponsor;
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

  const accentClass =
    campaign.state === 'funded'
      ? 'accent-funded'
      : campaign.state === 'finalized'
        ? 'accent-finalized'
        : campaign.state === 'completed'
          ? 'accent-completed'
          : 'accent-created';

  const allocationModeLabel =
    preview?.allocation_mode === 'code_impact'
      ? 'PR diff impact scoring'
      : 'Contributor metrics fallback';
  const previewWindowLabel =
    preview &&
    (preview.snapshot.contributor_source === 'repository_history_mvp'
      ? 'Full repository history'
      : `${formatDate(preview.snapshot.window_start)} to ${formatDate(preview.snapshot.window_end)}`);

  return (
    <div className="max-w-2xl mx-auto">
      <Link
        to="/"
        className="inline-flex items-center gap-1.5 text-xs text-gray-500 hover:text-white transition-colors mb-5"
      >
        ← Campaigns
      </Link>

      {/* Header */}
      <div className={`card ${accentClass} mb-5 animate-fade-in-up`}>
        <div className="flex items-start justify-between mb-4">
          <div>
            <h1 className="text-xl font-bold">{campaign.repo}</h1>
            <p className="text-[11px] text-gray-600 font-mono mt-1">
              {campaign.campaign_id}
              {campaign.owner_github_username && <> · @{campaign.owner_github_username}</>}
            </p>
          </div>
          <span className={`badge ${badgeClass} flex-shrink-0`}>{stateConfig.label}</span>
        </div>

        <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
          <div className="stat-block animate-fade-in" style={{ animationDelay: '60ms' }}>
            <span className="text-[10px] text-gray-600 uppercase tracking-wider">Pool</span>
            <p className="text-base font-bold text-solana-green mt-0.5">
              {formatSOL(campaign.pool_amount)} SOL
            </p>
          </div>
          <div className="stat-block animate-fade-in" style={{ animationDelay: '120ms' }}>
            <span className="text-[10px] text-gray-600 uppercase tracking-wider">Deadline</span>
            <p className="text-xs font-semibold mt-1">{formatDate(campaign.deadline)}</p>
          </div>
          <div className="stat-block animate-fade-in" style={{ animationDelay: '180ms' }}>
            <span className="text-[10px] text-gray-600 uppercase tracking-wider">Created</span>
            <p className="text-xs font-semibold mt-1">{formatDate(campaign.created_at)}</p>
          </div>
          <div className="stat-block animate-fade-in" style={{ animationDelay: '240ms' }}>
            <span className="text-[10px] text-gray-600 uppercase tracking-wider">Sponsor</span>
            <p className="text-[10px] font-mono mt-1 truncate">
              {campaign.sponsor || campaign.authority || 'N/A'}
            </p>
          </div>
        </div>

        {(isCompleted || isFinalized) && (
          <div className="mt-3 pt-3 border-t border-solana-border/50 flex items-center justify-between text-sm">
            <div>
              <span className="text-[10px] text-gray-600 uppercase tracking-wider">Claimed</span>
              <p className="font-semibold mt-0.5">
                {formatSOL(campaign.total_claimed)} / {formatSOL(campaign.pool_amount)} SOL
              </p>
            </div>
            {campaign.campaign_pda && (
              <div className="text-right">
                <span className="text-[10px] text-gray-600 uppercase tracking-wider">PDA</span>
                <p className="text-[10px] font-mono mt-0.5 truncate max-w-[180px]">
                  {campaign.campaign_pda}
                </p>
              </div>
            )}
          </div>
        )}

        {campaign.tx_signature && (
          <div className="mt-3 pt-3 border-t border-solana-border/50 flex items-center gap-2">
            <span className="text-[10px] text-gray-600 uppercase tracking-wider">Tx</span>
            <a
              href={`https://explorer.solana.com/tx/${campaign.tx_signature}?cluster=${import.meta.env.VITE_SOLANA_NETWORK || 'devnet'}`}
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs text-solana-purple hover:underline font-mono"
            >
              {campaign.tx_signature.slice(0, 20)}...
            </a>
          </div>
        )}
      </div>

      {/* GitHub App Install Banner */}
      {!isFinalized && !isCompleted && (
        <div className="card !border-blue-500/15 !bg-blue-500/[0.02] mb-5 flex items-center gap-3 !p-3">
          <span className="text-sm">🔔</span>
          <div className="flex-1 min-w-0">
            <p className="text-xs text-blue-300 font-medium">Auto-notify contributors</p>
            <p className="text-[10px] text-gray-600 mt-0.5">
              Install the GitHub App to post reward notifications on PRs.
            </p>
          </div>
          <a
            href="https://github.com/apps/repobounty-ai/installations/new"
            target="_blank"
            rel="noopener noreferrer"
            className="text-[10px] bg-blue-500/10 text-blue-300 hover:bg-blue-500/20 px-2.5 py-1 rounded-md transition-colors border border-blue-500/15 flex-shrink-0"
          >
            Install
          </a>
        </div>
      )}

      {/* Finalize Actions */}
      {canShowFinalizeCard && (
        <div className="card mb-5 animate-fade-in-up" style={{ animationDelay: '100ms' }}>
          <h2 className="text-sm font-semibold mb-2">Finalize Campaign</h2>
          <p className="text-xs text-gray-500 mb-3">
            Deadline passed. Preview saves a frozen snapshot; finalize commits it on-chain.
          </p>
          {!campaign.owner_github_username ? (
            <p className="text-xs text-yellow-200">
              Manual finalize unavailable — no creator account stored. The auto-finalize worker will
              handle it.
            </p>
          ) : !user ? (
            <p className="text-xs text-yellow-200">
              Log in as @{campaign.owner_github_username} to preview/finalize.
            </p>
          ) : !isOwner ? (
            <p className="text-xs text-yellow-200">
              Only @{campaign.owner_github_username} can preview or finalize.
            </p>
          ) : (
            <div className="space-y-3">
              {!solanaReady && (
                <div className="bg-yellow-500/5 border border-yellow-500/20 rounded-lg p-2.5 text-xs text-yellow-200">
                  Backend not connected to Solana. Preview is available but finalization is
                  disabled.
                </div>
              )}
              <div className="flex gap-2">
                <button
                  onClick={handlePreview}
                  disabled={previewing}
                  className="btn-secondary text-xs"
                >
                  {previewing ? 'Loading...' : 'Preview'}
                </button>
                {preview && (
                  <button
                    onClick={handleFinalize}
                    disabled={finalizing || !solanaReady}
                    className="btn-primary text-xs"
                  >
                    {finalizing
                      ? 'Finalizing...'
                      : solanaReady
                        ? 'Finalize on Solana'
                        : 'Solana Required'}
                  </button>
                )}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Sponsor wallet-proof fallback */}
      {canSponsorFinalize && (
        <div
          className="card mb-5 animate-fade-in-up !border-solana-purple/20"
          style={{ animationDelay: '120ms' }}
        >
          <h2 className="text-sm font-semibold mb-2">Sponsor Finalize</h2>
          <p className="text-xs text-gray-500 mb-3">
            The auto-finalize worker usually handles this automatically. If it hasn&apos;t run yet,
            you can finalize now by signing a proof with your wallet.
          </p>
          {!solanaReady && (
            <div className="bg-yellow-500/5 border border-yellow-500/20 rounded-lg p-2.5 text-xs text-yellow-200 mb-3">
              Backend not connected to Solana — finalization is disabled.
            </div>
          )}
          {!signMessage && (
            <p className="text-xs text-yellow-200 mb-2">
              Your wallet does not support message signing.
            </p>
          )}
          <button
            onClick={handleSponsorFinalize}
            disabled={sponsorFinalizing || !solanaReady || !signMessage}
            className="btn-primary text-xs"
          >
            {sponsorFinalizing ? 'Finalizing...' : 'Finalize now'}
          </button>
        </div>
      )}

      {/* Preview */}
      {preview && !isFinalized && (
        <div className="card mb-5 !border-yellow-500/15 animate-fade-in-up">
          <h2 className="text-sm font-semibold mb-1">AI Allocation Preview</h2>
          <p className="text-[10px] text-gray-600 mb-3">
            {preview.ai_model} · {allocationModeLabel} · v{preview.snapshot.version}
            {previewWindowLabel && <> · {previewWindowLabel}</>}
          </p>
          {preview.snapshot.contributor_notes && (
            <p className="text-[10px] text-gray-600 mb-3">{preview.snapshot.contributor_notes}</p>
          )}

          {preview.contributors.length > 0 && (
            <div className="mb-4">
              <h3 className="text-xs font-medium text-gray-500 mb-1.5">Contributors</h3>
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="text-left text-gray-600 border-b border-solana-border/50">
                      <th className="pb-1.5 font-medium">User</th>
                      <th className="pb-1.5 font-medium">Commits</th>
                      <th className="pb-1.5 font-medium">PRs</th>
                      <th className="pb-1.5 font-medium">Reviews</th>
                    </tr>
                  </thead>
                  <tbody>
                    {preview.contributors.map((c) => (
                      <tr key={c.username} className="border-b border-solana-border/30">
                        <td className="py-1.5 font-mono">@{c.username}</td>
                        <td className="py-1.5">{c.commits}</td>
                        <td className="py-1.5">{c.pull_requests}</td>
                        <td className="py-1.5">{c.reviews}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          <h3 className="text-xs font-medium text-gray-500 mb-2">Proposed Allocations</h3>
          <div className="space-y-2.5">
            {preview.allocations.map((a) => (
              <div key={a.contributor}>
                <div className="flex items-center justify-between text-xs mb-1">
                  <span className="font-mono">@{a.contributor}</span>
                  <span className="font-semibold">
                    {(a.percentage / 100).toFixed(1)}% · {formatSOL(a.amount)} SOL
                  </span>
                </div>
                <AllocationBar percentage={a.percentage} />
                {a.reasoning && <p className="text-[10px] text-gray-600 mt-0.5">{a.reasoning}</p>}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Finalized Allocations with Claim */}
      {isFinalized && campaign.allocations && campaign.allocations.length > 0 && (
        <div className="card animate-fade-in-up" style={{ animationDelay: '60ms' }}>
          <h2 className="text-sm font-semibold mb-1">Final Allocations</h2>
          <p className="text-[10px] text-gray-600 mb-4">
            Connect your wallet and sign a proof to claim your share.
          </p>
          {!solanaReady && (
            <div className="bg-yellow-500/5 border border-yellow-500/20 rounded-lg p-2.5 text-xs text-yellow-200 mb-3">
              Solana not connected — claiming is disabled.
            </div>
          )}
          <div className="space-y-2">
            {campaign.allocations.map((a) => {
              const isOwnAllocation = user?.github_username === a.contributor;
              const isCurrentlyClaiming = claiming === a.contributor;

              return (
                <div
                  key={a.contributor}
                  className={`p-3 rounded-lg border transition-all duration-200 ${
                    a.claimed
                      ? 'border-solana-green/20 bg-solana-green/[0.02]'
                      : 'border-solana-border hover:border-solana-border-light'
                  }`}
                >
                  <div className="flex items-center justify-between mb-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-xs font-medium">@{a.contributor}</span>
                      {a.claimed && (
                        <span className="badge badge-completed text-[10px]">Claimed</span>
                      )}
                    </div>
                    <span className="text-xs font-bold text-solana-green">
                      {(a.percentage / 100).toFixed(1)}% · {formatSOL(a.amount)} SOL
                    </span>
                  </div>
                  <AllocationBar percentage={a.percentage} />
                  {a.reasoning && <p className="text-[10px] text-gray-600 mt-1">{a.reasoning}</p>}
                  {isOwnAllocation && !a.claimed && (
                    <div className="mt-2 pt-2 border-t border-solana-border/30">
                      {!publicKey ? (
                        <button
                          onClick={() => setVisible(true)}
                          className="btn-primary text-[10px] !py-1 !px-3"
                        >
                          Connect Wallet
                        </button>
                      ) : !solanaReady ? (
                        <button disabled className="btn-primary text-[10px] !py-1 !px-3 opacity-50">
                          Solana Required
                        </button>
                      ) : (
                        <button
                          onClick={() => handleClaim(a.contributor)}
                          disabled={isCurrentlyClaiming}
                          className="btn-primary text-[10px] !py-1 !px-3"
                        >
                          {isCurrentlyClaiming ? 'Claiming...' : `Claim ${formatSOL(a.amount)} SOL`}
                        </button>
                      )}
                    </div>
                  )}
                  {a.claimed && a.claimant_wallet && (
                    <p className="text-[10px] text-gray-600 mt-1 font-mono">
                      → {a.claimant_wallet.slice(0, 8)}...{a.claimant_wallet.slice(-8)}
                    </p>
                  )}
                </div>
              );
            })}
          </div>

          {campaign.finalized_at && (
            <p className="text-[10px] text-gray-600 mt-4 pt-3 border-t border-solana-border/50">
              Finalized {formatDate(campaign.finalized_at)}
            </p>
          )}
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="bg-red-500/5 border border-red-500/15 rounded-lg p-3 text-xs text-red-400 mt-5">
          {error}
        </div>
      )}
    </div>
  );
}
