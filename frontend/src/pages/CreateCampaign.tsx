import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useWallet } from '@solana/wallet-adapter-react';
import { useConnection } from '@solana/wallet-adapter-react';
import { useWalletModal } from '@solana/wallet-adapter-react-ui';
import { VersionedTransaction } from '@solana/web3.js';
import bs58 from 'bs58';
import { api } from '../api/client';
import { useAuth } from '../hooks/useAuth';

function pad(value: number): string {
  return value.toString().padStart(2, '0');
}

function toDateTimeLocalValue(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
    date.getDate()
  )}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function toStableRFC3339(value: string): string | null {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return null;
  }
  return date.toISOString().replace(/\.\d{3}Z$/, 'Z');
}

export default function CreateCampaign() {
  const { publicKey, sendTransaction, signMessage } = useWallet();
  const { connection } = useConnection();
  const { setVisible } = useWalletModal();
  const { user, isLoading: authLoading, login } = useAuth();
  const navigate = useNavigate();

  const [step, setStep] = useState<1 | 2>(1);
  const [repo, setRepo] = useState('');
  const [poolSol, setPoolSol] = useState('');
  const [deadline, setDeadline] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [createdId, setCreatedId] = useState<string | null>(null);
  const [solanaReady, setSolanaReady] = useState(true);

  const minDeadline = toDateTimeLocalValue(new Date(Date.now() + 24 * 60 * 60 * 1000));

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

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    if (!user) {
      setError('Log in with GitHub to create a campaign.');
      return;
    }
    if (!publicKey) {
      setVisible(true);
      return;
    }
    if (!signMessage) {
      setError('This wallet does not support message signing.');
      return;
    }
    if (!solanaReady) {
      setError('Campaign creation is unavailable until the backend is connected to Solana.');
      return;
    }

    if (!repo.match(/^[a-zA-Z0-9._-]+\/[a-zA-Z0-9._-]+$/)) {
      setError('Repository must be in "owner/repo" format');
      return;
    }

    const poolLamports = Math.round(parseFloat(poolSol) * 1e9);
    if (isNaN(poolLamports) || poolLamports <= 0) {
      setError('Pool amount must be a positive number');
      return;
    }
    if (parseFloat(poolSol) > 10000) {
      setError('Pool amount cannot exceed 10,000 SOL');
      return;
    }

    const deadlineRFC3339 = toStableRFC3339(deadline);
    if (!deadlineRFC3339) {
      setError('Deadline must include a valid date and time');
      return;
    }

    setSubmitting(true);
    try {
      const challenge = await api.createCampaignChallenge({
        repo,
        pool_amount: poolLamports,
        deadline: deadlineRFC3339,
        sponsor_wallet: publicKey.toBase58(),
      });
      const signatureBytes = await signMessage(new TextEncoder().encode(challenge.message));

      const result = await api.createCampaign({
        repo,
        pool_amount: poolLamports,
        deadline: deadlineRFC3339,
        sponsor_wallet: publicKey.toBase58(),
        challenge_id: challenge.challenge_id,
        signature: bs58.encode(signatureBytes),
      });
      setCreatedId(result.campaign_id);
      setStep(2);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to create campaign');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleFund() {
    if (!publicKey || !createdId) return;
    setError(null);

    if (!solanaReady) {
      setError('Funding is unavailable until the backend is connected to Solana.');
      return;
    }

    setSubmitting(true);

    try {
      const fundTx = await api.fundTx(createdId, publicKey.toBase58());
      const txBytes = bs58.decode(fundTx.transaction);
      const transaction = VersionedTransaction.deserialize(new Uint8Array(txBytes));

      const signature = await sendTransaction(transaction, connection);
      await waitForSignature(signature);

      navigate(`/campaign/${createdId}`);
    } catch (err: unknown) {
      if (err instanceof Error) {
        if (err.message.includes('User rejected')) {
          setError('Transaction rejected by wallet');
        } else {
          setError(err.message);
        }
      } else {
        setError('Failed to send fund transaction');
      }
    } finally {
      setSubmitting(false);
    }
  }

  async function waitForSignature(signature: string) {
    await connection.confirmTransaction(signature, 'confirmed');
  }

  function handleBack() {
    if (step === 2 && createdId) {
      navigate(`/campaign/${createdId}`);
      return;
    }
    navigate('/');
  }

  return (
    <div className="max-w-xl mx-auto">
      <h1 className="text-3xl font-bold mb-2">
        <span className="gradient-text">Create Campaign</span>
      </h1>
      <p className="text-gray-400 mb-8">
        Create a campaign as your GitHub account, prove control of the sponsor wallet with a signed
        message, then fund it from the connected wallet.
      </p>

      {!solanaReady && (
        <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-4 text-sm text-yellow-200 mb-6">
          The backend is not connected to Solana right now. Creating and funding campaigns is
          disabled until a real authority key and program ID are configured.
        </div>
      )}

      <div className="flex gap-2 mb-8">
        <div
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium ${
            step >= 1 ? 'bg-solana-purple/20 text-solana-purple' : 'bg-solana-dark text-gray-500'
          }`}
        >
          <span className="w-6 h-6 rounded-full bg-solana-purple text-white flex items-center justify-center text-xs">
            1
          </span>
          Details
        </div>
        <div className="flex-1 border-t border-solana-border self-center" />
        <div
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium ${
            step >= 2 ? 'bg-solana-green/20 text-solana-green' : 'bg-solana-dark text-gray-500'
          }`}
        >
          <span className="w-6 h-6 rounded-full bg-solana-green text-white flex items-center justify-center text-xs">
            2
          </span>
          Fund
        </div>
      </div>

      {step === 1 && (
        <form onSubmit={handleCreate} className="space-y-6">
          <div>
            <label className="block text-sm font-medium mb-2">GitHub Repository</label>
            <input
              type="text"
              value={repo}
              onChange={(e) => setRepo(e.target.value)}
              placeholder="owner/repo"
              className="input"
              required
            />
            <p className="text-xs text-gray-500 mt-1">
              Public repository in &quot;owner/repo&quot; format
            </p>
          </div>

          <div className="bg-solana-card border border-solana-border rounded-lg p-4 text-sm">
            <p className="text-gray-300">
              Campaign owner:
              <span className="ml-2 font-medium text-white">
                {user ? `@${user.github_username}` : 'GitHub login required'}
              </span>
            </p>
            <p className="text-xs text-gray-500 mt-2">
              Manual funding and finalization stay attached to the GitHub account that created the
              campaign, and the connected wallet must sign the create proof before the backend
              accepts it.
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">Reward Pool (SOL)</label>
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
              type="datetime-local"
              value={deadline}
              onChange={(e) => setDeadline(e.target.value)}
              min={minDeadline}
              step="60"
              className="input"
              required
            />
            <p className="text-xs text-gray-500 mt-1">
              Choose a deadline at least 24 hours from now
            </p>
          </div>

          {error && (
            <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-4 text-sm text-red-400">
              {error}
            </div>
          )}

          <div className="flex gap-4 pt-2">
            {!user ? (
              <button
                type="button"
                onClick={() => void login()}
                className="btn-primary flex-1"
                disabled={authLoading}
              >
                {authLoading ? 'Checking session...' : 'Log in with GitHub'}
              </button>
            ) : !publicKey ? (
              <button type="button" onClick={() => setVisible(true)} className="btn-primary flex-1">
                Connect Wallet to Continue
              </button>
            ) : !solanaReady ? (
              <button
                type="button"
                disabled
                className="btn-primary flex-1 opacity-60 cursor-not-allowed"
              >
                Solana Backend Required
              </button>
            ) : (
              <button type="submit" disabled={submitting} className="btn-primary flex-1">
                {submitting ? 'Creating...' : 'Create Campaign'}
              </button>
            )}
            <button type="button" onClick={handleBack} className="btn-secondary">
              Cancel
            </button>
          </div>
        </form>
      )}

      {step === 2 && (
        <div className="space-y-6">
          <div className="bg-solana-card border border-solana-border rounded-lg p-6 space-y-3">
            <div className="flex justify-between text-sm">
              <span className="text-gray-400">Repository</span>
              <span className="font-mono">{repo}</span>
            </div>
            <div className="flex justify-between text-sm">
              <span className="text-gray-400">Reward Pool</span>
              <span className="font-mono">{poolSol} SOL</span>
            </div>
            <div className="flex justify-between text-sm">
              <span className="text-gray-400">Deadline</span>
              <span>{deadline}</span>
            </div>
            {user && (
              <div className="flex justify-between text-sm">
                <span className="text-gray-400">Owner</span>
                <span>@{user.github_username}</span>
              </div>
            )}
          </div>

          <div className="bg-solana-purple/10 border border-solana-purple/30 rounded-lg p-6 text-center">
            <p className="text-sm text-gray-300 mb-2">
              You will be prompted to sign a transaction in your wallet to fund the escrow vault.
            </p>
            <p className="text-2xl font-bold text-solana-purple">{poolSol} SOL</p>
          </div>

          <p className="text-xs text-gray-500">
            The connected wallet signs the funding transaction, but future manual management stays
            limited to the GitHub account that created this campaign.
          </p>

          {error && (
            <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-4 text-sm text-red-400">
              {error}
            </div>
          )}

          <div className="flex gap-4 pt-2">
            <button
              type="button"
              onClick={handleFund}
              disabled={submitting || !publicKey || !solanaReady}
              className="btn-primary flex-1"
            >
              {submitting
                ? 'Confirming...'
                : solanaReady
                  ? 'Fund Campaign'
                  : 'Solana Backend Required'}
            </button>
            <button
              type="button"
              onClick={handleBack}
              className="btn-secondary"
              disabled={submitting}
            >
              Skip for Now
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
