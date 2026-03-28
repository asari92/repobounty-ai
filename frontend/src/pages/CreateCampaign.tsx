import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useWallet } from "@solana/wallet-adapter-react";
import { useWalletModal } from "@solana/wallet-adapter-react-ui";
import { api } from "../api/client";

export default function CreateCampaign() {
  const { publicKey } = useWallet();
  const { setVisible } = useWalletModal();
  const navigate = useNavigate();

  const [repo, setRepo] = useState("");
  const [poolSol, setPoolSol] = useState("");
  const [deadline, setDeadline] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const tomorrow = new Date(Date.now() + 86400000).toISOString().split("T")[0];

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    if (!publicKey) {
      setVisible(true);
      return;
    }

    if (!repo.match(/^[a-zA-Z0-9._-]+\/[a-zA-Z0-9._-]+$/)) {
      setError('Repository must be in "owner/repo" format');
      return;
    }

    const poolLamports = Math.round(parseFloat(poolSol) * 1e9);
    if (isNaN(poolLamports) || poolLamports <= 0) {
      setError("Pool amount must be a positive number");
      return;
    }

    setSubmitting(true);
    try {
      const result = await api.createCampaign({
        repo,
        pool_amount: poolLamports,
        deadline: new Date(deadline).toISOString(),
        wallet_address: publicKey.toBase58(),
      });
      navigate(`/campaign/${result.campaign_id}`);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to create campaign");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="max-w-xl mx-auto">
      <h1 className="text-3xl font-bold mb-2">
        <span className="gradient-text">Create Campaign</span>
      </h1>
      <p className="text-gray-400 mb-8">
        Fund a public GitHub repository and let AI allocate rewards to
        contributors
      </p>

      <form onSubmit={handleSubmit} className="space-y-6">
        <div>
          <label className="block text-sm font-medium mb-2">
            GitHub Repository
          </label>
          <input
            type="text"
            value={repo}
            onChange={(e) => setRepo(e.target.value)}
            placeholder="owner/repo"
            className="input"
            required
          />
          <p className="text-xs text-gray-500 mt-1">
            Public repository in "owner/repo" format
          </p>
        </div>

        <div>
          <label className="block text-sm font-medium mb-2">
            Reward Pool (SOL)
          </label>
          <input
            type="number"
            value={poolSol}
            onChange={(e) => setPoolSol(e.target.value)}
            placeholder="1.0"
            step="0.01"
            min="0.01"
            className="input"
            required
          />
        </div>

        <div>
          <label className="block text-sm font-medium mb-2">Deadline</label>
          <input
            type="date"
            value={deadline}
            onChange={(e) => setDeadline(e.target.value)}
            min={tomorrow}
            className="input"
            required
          />
          <p className="text-xs text-gray-500 mt-1">
            After this date, the campaign can be finalized
          </p>
        </div>

        {error && (
          <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-4 text-sm text-red-400">
            {error}
          </div>
        )}

        <div className="flex gap-4 pt-2">
          {!publicKey ? (
            <button
              type="button"
              onClick={() => setVisible(true)}
              className="btn-primary flex-1"
            >
              Connect Wallet to Continue
            </button>
          ) : (
            <button
              type="submit"
              disabled={submitting}
              className="btn-primary flex-1"
            >
              {submitting ? "Creating..." : "Create Campaign"}
            </button>
          )}
          <button
            type="button"
            onClick={() => navigate("/")}
            className="btn-secondary"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
