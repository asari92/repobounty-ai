import { useEffect, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { useWallet } from "@solana/wallet-adapter-react";
import { useWalletModal } from "@solana/wallet-adapter-react-ui";
import { api } from "../api/client";
import { useAuth } from "../hooks/useAuth";
import type { Campaign, FinalizePreviewResponse } from "../types";

function formatSOL(lamports: number): string {
  return (lamports / 1e9).toFixed(4);
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString("en-US", {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

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

function getStateConfig(state: Campaign["state"]) {
  switch (state) {
    case "completed":
      return { label: "Completed", classes: "bg-solana-green/20 text-solana-green" };
    case "finalized":
      return { label: "Finalized", classes: "bg-solana-green/20 text-solana-green" };
    case "funded":
      return { label: "Funded", classes: "bg-blue-500/20 text-blue-400" };
    case "created":
      return { label: "Created", classes: "bg-solana-purple/20 text-solana-purple" };
    default:
      return { label: state, classes: "bg-solana-purple/20 text-solana-purple" };
  }
}

export default function CampaignDetails() {
  const { id } = useParams<{ id: string }>();
  const { publicKey } = useWallet();
  const { setVisible } = useWalletModal();
  const { user } = useAuth();
  const [campaign, setCampaign] = useState<Campaign | null>(null);
  const [preview, setPreview] = useState<FinalizePreviewResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [finalizing, setFinalizing] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [claiming, setClaiming] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    api
      .getCampaign(id)
      .then(setCampaign)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [id]);

  async function handlePreview() {
    if (!id) return;
    setPreviewing(true);
    setError(null);
    try {
      const result = await api.finalizePreview(id);
      setPreview(result);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Preview failed");
    } finally {
      setPreviewing(false);
    }
  }

  async function handleFinalize() {
    if (!id) return;
    setFinalizing(true);
    setError(null);
    try {
      const result = await api.finalize(id);
      setCampaign((prev) =>
        prev
          ? {
              ...prev,
              state: "finalized",
              allocations: result.allocations,
              tx_signature: result.tx_signature,
            }
          : null
      );
      setPreview(null);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Finalization failed");
    } finally {
      setFinalizing(false);
    }
  }

  async function handleClaim(contributor: string) {
    if (!id) return;
    setClaiming(contributor);
    setError(null);
    try {
      await api.claimAllocation(id, contributor, publicKey?.toBase58() || "");
      setCampaign((prev) => {
        if (!prev) return null;
        return {
          ...prev,
          allocations: prev.allocations.map((a) =>
            a.contributor === contributor
              ? { ...a, claimed: true, claimant_wallet: publicKey?.toBase58() || "" }
              : a
          ),
          total_claimed: prev.total_claimed + (prev.allocations.find((a) => a.contributor === contributor)?.amount || 0),
          state: prev.allocations.every((a) => a.contributor === contributor ? true : a.claimed) ? "completed" : prev.state,
        };
      });
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Claim failed");
    } finally {
      setClaiming(null);
    }
  }

  if (loading) {
    return (
      <div className="text-center py-20">
        <div className="inline-block w-8 h-8 border-2 border-solana-purple border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!campaign) {
    return (
      <div className="card text-center py-12">
        <p className="text-red-400">Campaign not found</p>
        <Link to="/" className="btn-secondary text-sm mt-4 inline-block">
          Back to Campaigns
        </Link>
      </div>
    );
  }

  const isFinalized = campaign.state === "finalized" || campaign.state === "completed";
  const isCompleted = campaign.state === "completed";
  const isPastDeadline = new Date(campaign.deadline) < new Date();
  const canFinalize = campaign.state === "created" && isPastDeadline;
  const stateConfig = getStateConfig(campaign.state);

  return (
    <div className="max-w-3xl mx-auto">
      <Link
        to="/"
        className="text-sm text-gray-400 hover:text-white transition-colors mb-6 inline-block"
      >
        &larr; Back to Campaigns
      </Link>

      {/* Header */}
      <div className="card mb-6">
        <div className="flex items-start justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold">{campaign.repo}</h1>
            <p className="text-sm text-gray-400 mt-1 font-mono">
              {campaign.campaign_id}
            </p>
            {campaign.sponsor && campaign.sponsor !== "" && (
              <p className="text-xs text-gray-500 mt-0.5">
                Sponsored by {campaign.sponsor.slice(0, 8)}...
              </p>
            )}
          </div>
          <span
            className={`text-sm font-semibold px-4 py-1.5 rounded-full ${stateConfig.classes}`}
          >
            {stateConfig.label}
          </span>
        </div>

        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div>
            <span className="text-xs text-gray-400 uppercase tracking-wide">Pool</span>
            <p className="text-lg font-bold text-solana-green">
              {formatSOL(campaign.pool_amount)} SOL
            </p>
          </div>
          <div>
            <span className="text-xs text-gray-400 uppercase tracking-wide">Deadline</span>
            <p className="text-lg font-semibold">{formatDate(campaign.deadline)}</p>
          </div>
          <div>
            <span className="text-xs text-gray-400 uppercase tracking-wide">Created</span>
            <p className="text-lg font-semibold">{formatDate(campaign.created_at)}</p>
          </div>
          <div>
            <span className="text-xs text-gray-400 uppercase tracking-wide">Sponsor</span>
            <p className="text-sm font-mono mt-1 truncate">
              {campaign.sponsor || campaign.authority || "N/A"}
            </p>
          </div>
        </div>

        {(isCompleted || isFinalized) && (
          <div className="mt-4 pt-4 border-t border-solana-border grid grid-cols-2 gap-4">
            <div>
              <span className="text-xs text-gray-400 uppercase tracking-wide">Claimed</span>
              <p className="text-lg font-semibold">
                {formatSOL(campaign.total_claimed)} / {formatSOL(campaign.pool_amount)} SOL
              </p>
            </div>
            {campaign.campaign_pda && (
              <div>
                <span className="text-xs text-gray-400 uppercase tracking-wide">Campaign PDA</span>
                <p className="text-sm font-mono mt-1 truncate">{campaign.campaign_pda}</p>
              </div>
            )}
          </div>
        )}

        {campaign.tx_signature && (
          <div className="mt-4 pt-4 border-t border-solana-border">
            <span className="text-xs text-gray-400">Transaction: </span>
            <a
              href={`https://explorer.solana.com/tx/${campaign.tx_signature}?cluster=devnet`}
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
        <div className="card mb-6 border-blue-500/30 bg-blue-500/5">
          <div className="flex items-start gap-3">
            <div className="text-2xl mt-0.5">🔔</div>
            <div className="flex-1">
              <h3 className="text-sm font-semibold text-blue-300">Auto-notify contributors</h3>
              <p className="text-xs text-gray-400 mt-1">
                Install the RepoBounty GitHub App to automatically post reward notifications on
                contributors' PRs after finalization.
              </p>
              <a
                href="https://github.com/apps/repobounty-ai/installations/new"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-block mt-2 text-xs bg-blue-500/20 text-blue-300 hover:bg-blue-500/30 px-3 py-1.5 rounded-full transition-colors"
              >
                Install GitHub App
              </a>
            </div>
          </div>
        </div>
      )}

      {/* Finalize Actions */}
      {canFinalize && (
        <div className="card mb-6">
          <h2 className="text-lg font-semibold mb-4">Finalize Campaign</h2>
          <p className="text-sm text-gray-400 mb-4">
            The deadline has passed. Fetch contributor data and generate
            AI-powered reward allocations.
          </p>
          <div className="flex gap-3">
            <button
              onClick={handlePreview}
              disabled={previewing}
              className="btn-secondary text-sm"
            >
              {previewing ? "Loading preview..." : "Preview Allocations"}
            </button>
            {preview && (
              <button
                onClick={handleFinalize}
                disabled={finalizing}
                className="btn-primary text-sm"
              >
                {finalizing ? "Finalizing on Solana..." : "Finalize on Solana"}
              </button>
            )}
          </div>
        </div>
      )}

      {/* Preview */}
      {preview && !isFinalized && (
        <div className="card mb-6 border-yellow-500/30">
          <h2 className="text-lg font-semibold mb-1">AI Allocation Preview</h2>
          <p className="text-xs text-gray-400 mb-4">
            Model: {preview.ai_model}
          </p>

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
                {a.reasoning && (
                  <p className="text-xs text-gray-500 mt-1">{a.reasoning}</p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Finalized Allocations with Claim Buttons */}
      {isFinalized && campaign.allocations.length > 0 && (
        <div className="card">
          <h2 className="text-lg font-semibold mb-4">Final Allocations (On-Chain)</h2>
          <div className="space-y-4">
            {campaign.allocations.map((a) => {
              const isOwnAllocation = user?.github_username === a.contributor;
              const isCurrentlyClaiming = claiming === a.contributor;

              return (
                <div
                  key={a.contributor}
                  className={`p-4 rounded-lg border ${
                    a.claimed ? "border-solana-green/30 bg-solana-green/5" : "border-solana-border"
                  }`}
                >
                  <div className="flex items-center justify-between mb-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono font-medium">@{a.contributor}</span>
                      {a.claimed && (
                        <span className="text-xs bg-solana-green/20 text-solana-green px-2 py-0.5 rounded-full">
                          Claimed
                        </span>
                      )}
                    </div>
                    <span className="font-bold text-solana-green">
                      {(a.percentage / 100).toFixed(1)}% &middot; {formatSOL(a.amount)} SOL
                    </span>
                  </div>
                  <AllocationBar percentage={a.percentage} />
                  {a.reasoning && (
                    <p className="text-xs text-gray-500 mt-1">{a.reasoning}</p>
                  )}
                  {isOwnAllocation && !a.claimed && (
                    <div className="mt-3 pt-3 border-t border-solana-border/50">
                      {!publicKey ? (
                        <button
                          onClick={() => setVisible(true)}
                          className="btn-primary text-xs py-1.5 px-3"
                        >
                          Connect Wallet to Claim
                        </button>
                      ) : (
                        <button
                          onClick={() => handleClaim(a.contributor)}
                          disabled={isCurrentlyClaiming}
                          className="btn-primary text-xs py-1.5 px-3"
                        >
                          {isCurrentlyClaiming ? "Claiming..." : `Claim ${formatSOL(a.amount)} SOL`}
                        </button>
                      )}
                    </div>
                  )}
                  {a.claimed && a.claimant_wallet && (
                    <p className="text-xs text-gray-500 mt-2">
                      Claimed by {a.claimant_wallet.slice(0, 8)}...{a.claimant_wallet.slice(-8)}
                    </p>
                  )}
                </div>
              );
            })}
          </div>

          {campaign.finalized_at && (
            <p className="text-xs text-gray-500 mt-6 pt-4 border-t border-solana-border">
              Finalized at {formatDate(campaign.finalized_at)}
            </p>
          )}
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="card border-red-500/30 bg-red-500/10 mt-6">
          <p className="text-sm text-red-400">{error}</p>
        </div>
      )}
    </div>
  );
}