import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useWallet } from '@solana/wallet-adapter-react';
import { useConnection } from '@solana/wallet-adapter-react';
import { useWalletModal } from '@solana/wallet-adapter-react-ui';
import { Transaction } from '@solana/web3.js';
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
      if (!challenge.challenge_id?.trim()) {
        throw new Error('Wallet proof challenge was not returned by the backend.');
      }

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
      const transaction = Transaction.from(txBytes);

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
    <div className="max-w-lg mx-auto">
      {/* Header */}
      <div className="mb-6 animate-fade-in-up">
        <h1 className="text-2xl font-bold tracking-tight mb-1">
          <span className="gradient-text">Create Campaign</span>
        </h1>
        <p className="text-xs text-gray-500">
          Fund a GitHub repo and let AI allocate rewards to contributors
        </p>
        <div className="gradient-line mt-3" />
      </div>

      {/* Step indicator */}
      <div className="flex items-center gap-3 mb-6 animate-fade-in" style={{ animationDelay: '60ms' }}>
        <div
          className={`flex items-center gap-2 text-xs font-medium ${
            step >= 1 ? 'text-solana-purple' : 'text-gray-600'
          }`}
        >
          <span className={`w-5 h-5 rounded-md flex items-center justify-center text-[10px] font-bold ${
            step >= 1 ? 'bg-solana-purple text-white' : 'bg-solana-border text-gray-500'
          }`}>
            1
          </span>
          Details
        </div>
        <div className="flex-1 h-px bg-solana-border" />
        <div
          className={`flex items-center gap-2 text-xs font-medium ${
            step >= 2 ? 'text-solana-green' : 'text-gray-600'
          }`}
        >
          <span className={`w-5 h-5 rounded-md flex items-center justify-center text-[10px] font-bold ${
            step >= 2 ? 'bg-solana-green text-white' : 'bg-solana-border text-gray-500'
          }`}>
            2
          </span>
          Fund
        </div>
      </div>

      {!solanaReady && (
        <div className="bg-yellow-500/5 border border-yellow-500/20 rounded-lg p-3 text-xs text-yellow-200 mb-5">
          Backend is not connected to Solana. Creating and funding campaigns is disabled.
        </div>
      )}

      {step === 1 && (
        <div className="animate-fade-in-up" style={{ animationDelay: '100ms' }}>
          <form onSubmit={handleCreate} className="space-y-4">
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-1.5">
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
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs font-medium text-gray-400 mb-1.5">
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
                <label className="block text-xs font-medium text-gray-400 mb-1.5">
                  Deadline
                </label>
                <input
                  type="datetime-local"
                  value={deadline}
                  onChange={(e) => setDeadline(e.target.value)}
                  min={minDeadline}
                  step="60"
                  className="input"
                  required
                />
              </div>
            </div>

            {/* Info box */}
            <div className="card !p-4 !bg-solana-green/[0.03] !border-solana-green/15 text-xs text-gray-400 space-y-1.5">
              <p><span className="text-solana-green font-medium">AI analysis</span> — commits and PRs are scored automatically</p>
              <p><span className="text-solana-green font-medium">Escrow</span> — funds held in Solana smart contract until deadline</p>
              <p><span className="text-solana-green font-medium">Merged code only</span> — only main branch contributions qualify</p>
            </div>

            {error && (
              <div className="bg-red-500/5 border border-red-500/15 rounded-lg p-3 text-xs text-red-400">
                {error}
              </div>
            )}

            <div className="flex gap-3 pt-1">
              {!user ? (
                <button
                  type="button"
                  onClick={() => void login()}
                  className="btn-primary flex-1"
                  disabled={authLoading}
                >
                  {authLoading ? 'Checking...' : 'Log in with GitHub'}
                </button>
              ) : !publicKey ? (
                <button
                  type="button"
                  onClick={() => setVisible(true)}
                  className="btn-primary flex-1"
                >
                  Connect Wallet
                </button>
              ) : !solanaReady ? (
                <button
                  type="button"
                  disabled
                  className="btn-primary flex-1"
                >
                  Solana Required
                </button>
              ) : (
                <button
                  type="submit"
                  disabled={submitting}
                  className="btn-primary flex-1"
                >
                  {submitting ? 'Creating...' : 'Create Campaign →'}
                </button>
              )}
              <button type="button" onClick={handleBack} className="btn-secondary">
                Cancel
              </button>
            </div>
          </form>

          {user && (
            <div className="flex items-center gap-2 mt-4 text-xs text-gray-600">
              <img
                src={user.avatar_url}
                alt={user.github_username}
                className="w-5 h-5 rounded-full"
              />
              Creating as @{user.github_username}
            </div>
          )}
        </div>
      )}

      {step === 2 && (
        <div className="animate-fade-in-up space-y-4">
          <div className="card">
            <h3 className="text-xs font-medium text-gray-500 uppercase tracking-wider mb-3">
              Summary
            </h3>
            <div className="space-y-2 text-sm">
              <div className="flex justify-between py-1.5 border-b border-solana-border/30">
                <span className="text-gray-500">Repository</span>
                <span className="font-mono text-white">{repo}</span>
              </div>
              <div className="flex justify-between py-1.5 border-b border-solana-border/30">
                <span className="text-gray-500">Pool</span>
                <span className="font-mono text-solana-green">{poolSol} SOL</span>
              </div>
              <div className="flex justify-between py-1.5 border-b border-solana-border/30">
                <span className="text-gray-500">Deadline</span>
                <span className="text-white">{deadline}</span>
              </div>
              {user && (
                <div className="flex justify-between py-1.5">
                  <span className="text-gray-500">Owner</span>
                  <span className="text-white">@{user.github_username}</span>
                </div>
              )}
            </div>
          </div>

          <div className="stat-block text-center py-6">
            <p className="text-xs text-gray-500 mb-2">Sign a wallet transaction to fund the escrow</p>
            <p className="text-3xl font-bold text-solana-purple">{poolSol} SOL</p>
          </div>

          <p className="text-[10px] text-gray-600 text-center">
            The connected wallet signs the funding transaction, but future management stays
            limited to the GitHub account that created this campaign.
          </p>

          {error && (
            <div className="bg-red-500/5 border border-red-500/15 rounded-lg p-3 text-xs text-red-400">
              {error}
            </div>
          )}

          <div className="flex gap-3">
            <button
              type="button"
              onClick={handleFund}
              disabled={submitting || !publicKey || !solanaReady}
              className="btn-primary flex-1"
            >
              {submitting
                ? 'Confirming...'
                : solanaReady
                  ? 'Fund Campaign →'
                  : 'Solana Required'}
            </button>
            <button
              type="button"
              onClick={handleBack}
              className="btn-secondary"
              disabled={submitting}
            >
              Skip
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
