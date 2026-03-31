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

function GitHubIcon() {
  return (
    <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
      <path
        fillRule="evenodd"
        d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"
        clipRule="evenodd"
      />
    </svg>
  );
}

function SolIcon() {
  return (
    <svg className="w-5 h-5" viewBox="0 0 24 24" fill="none">
      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="1.5" />
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
  );
}

function CalendarIcon() {
  return (
    <svg
      className="w-5 h-5"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={1.5}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 012.25-2.25h13.5A2.25 2.25 0 0121 7.5v11.25m-18 0A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75m-18 0v-7.5A2.25 2.25 0 015.25 9h13.5A2.25 2.25 0 0121 11.25v7.5"
      />
    </svg>
  );
}

function ArrowRightIcon() {
  return (
    <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3" />
    </svg>
  );
}

function CheckCircleIcon({ className }: { className?: string }) {
  return (
    <svg className={className || 'w-5 h-5'} viewBox="0 0 24 24" fill="none">
      <circle cx="12" cy="12" r="10" fill="currentColor" fillOpacity="0.15" />
      <path
        d="M9 12l2 2 4-4"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
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
    <div>
      {/* Title */}
      <div className="mb-8 animate-fade-in-up">
        <h1 className="text-4xl font-bold tracking-tight mb-2">
          <span className="gradient-text">Create Campaign</span>
        </h1>
        <p className="text-gray-400">
          Fund a public GitHub repository and let AI allocate rewards to contributors
        </p>
      </div>

      {/* Step indicator */}
      <div className="flex items-center gap-4 mb-8 animate-fade-in-up" style={{ animationDelay: '75ms' }}>
        <div
          className={`flex items-center gap-2.5 px-5 py-2.5 rounded-xl text-sm font-medium transition-all ${
            step >= 1
              ? 'bg-solana-purple/15 text-solana-purple border border-solana-purple/20'
              : 'bg-solana-card text-gray-500 border border-solana-border'
          }`}
        >
          <span className="w-7 h-7 rounded-full bg-solana-purple text-white flex items-center justify-center text-xs font-bold">
            1
          </span>
          Details
        </div>
        <div className="flex-1 border-t border-dashed border-solana-border" />
        <div
          className={`flex items-center gap-2.5 px-5 py-2.5 rounded-xl text-sm font-medium transition-all ${
            step >= 2
              ? 'bg-solana-green/15 text-solana-green border border-solana-green/20'
              : 'bg-solana-card text-gray-500 border border-solana-border'
          }`}
        >
          <span
            className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold ${
              step >= 2 ? 'bg-solana-green text-white' : 'bg-solana-border text-gray-400'
            }`}
          >
            2
          </span>
          Fund
        </div>
      </div>

      {!solanaReady && (
        <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-xl p-4 text-sm text-yellow-200 mb-6">
          The backend is not connected to Solana right now. Creating and funding campaigns is
          disabled until a real authority key and program ID are configured.
        </div>
      )}

      {step === 1 && (
        <div className="grid grid-cols-1 lg:grid-cols-5 gap-8">
          {/* Form */}
          <div className="lg:col-span-3 animate-fade-in-up" style={{ animationDelay: '150ms' }}>
            <form onSubmit={handleCreate} className="space-y-6">
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2 uppercase tracking-wide">
                  GitHub Repository
                </label>
                <div className="relative">
                  <div className="absolute inset-y-0 left-3.5 flex items-center pointer-events-none text-gray-500">
                    <GitHubIcon />
                  </div>
                  <input
                    type="text"
                    value={repo}
                    onChange={(e) => setRepo(e.target.value)}
                    placeholder="owner/repo"
                    className="input input-with-icon"
                    required
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-2 uppercase tracking-wide">
                    Reward Pool (SOL)
                  </label>
                  <div className="relative">
                    <div className="absolute inset-y-0 left-3.5 flex items-center pointer-events-none text-solana-green">
                      <SolIcon />
                    </div>
                    <input
                      type="number"
                      value={poolSol}
                      onChange={(e) => setPoolSol(e.target.value)}
                      placeholder="1.0"
                      step="0.01"
                      min="0.01"
                      className="input input-with-icon"
                      required
                    />
                  </div>
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-2 uppercase tracking-wide">
                    Deadline
                  </label>
                  <div className="relative">
                    <div className="absolute inset-y-0 left-3.5 flex items-center pointer-events-none text-gray-500">
                      <CalendarIcon />
                    </div>
                    <input
                      type="datetime-local"
                      value={deadline}
                      onChange={(e) => setDeadline(e.target.value)}
                      min={minDeadline}
                      step="60"
                      className="input input-with-icon"
                      required
                    />
                  </div>
                </div>
              </div>

              {error && (
                <div className="bg-red-500/10 border border-red-500/30 rounded-xl p-4 text-sm text-red-400">
                  {error}
                </div>
              )}

              <div className="flex gap-4 pt-2">
                {!user ? (
                  <button
                    type="button"
                    onClick={() => void login()}
                    className="btn-primary flex-1 flex items-center justify-center gap-2"
                    disabled={authLoading}
                  >
                    <GitHubIcon />
                    {authLoading ? 'Checking session...' : 'Log in with GitHub'}
                  </button>
                ) : !publicKey ? (
                  <button
                    type="button"
                    onClick={() => setVisible(true)}
                    className="btn-primary flex-1 flex items-center justify-center gap-3"
                  >
                    Connect Wallet to Continue
                    <ArrowRightIcon />
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
                  <button
                    type="submit"
                    disabled={submitting}
                    className="btn-primary flex-1 flex items-center justify-center gap-3"
                  >
                    {submitting ? 'Creating...' : 'Create Campaign'}
                    {!submitting && <ArrowRightIcon />}
                  </button>
                )}
                <button type="button" onClick={handleBack} className="btn-secondary">
                  Cancel
                </button>
              </div>
            </form>
          </div>

          {/* How it works sidebar */}
          <div className="lg:col-span-2 animate-fade-in-up" style={{ animationDelay: '250ms' }}>
            <div className="card border-solana-green/20 bg-solana-green/[0.03]">
              <h3 className="text-lg font-semibold text-solana-green mb-5">How it works</h3>
              <div className="space-y-5">
                <div className="flex items-start gap-3">
                  <CheckCircleIcon className="w-6 h-6 text-solana-green flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm text-gray-200 font-medium">AI-powered analysis</p>
                    <p className="text-xs text-gray-500 mt-1">
                      The AI analyzes pull requests and commits to determine impact scores for each
                      contributor automatically.
                    </p>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <CheckCircleIcon className="w-6 h-6 text-solana-green flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm text-gray-200 font-medium">Secure escrow</p>
                    <p className="text-xs text-gray-500 mt-1">
                      Funds are held in a secure Solana smart contract until the campaign deadline is
                      reached.
                    </p>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <CheckCircleIcon className="w-6 h-6 text-solana-green flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm text-gray-200 font-medium">Merged code only</p>
                    <p className="text-xs text-gray-500 mt-1">
                      Only merged code from the specified repository main branch is eligible for
                      rewards.
                    </p>
                  </div>
                </div>
              </div>
            </div>

            {user && (
              <div className="card mt-4">
                <div className="flex items-center gap-3">
                  <img
                    src={user.avatar_url}
                    alt={user.github_username}
                    className="w-8 h-8 rounded-full ring-2 ring-solana-border"
                  />
                  <div>
                    <p className="text-sm text-gray-300 font-medium">@{user.github_username}</p>
                    <p className="text-xs text-gray-500">Campaign creator</p>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {step === 2 && (
        <div className="max-w-xl mx-auto space-y-6 animate-fade-in-up">
          <div className="card space-y-4">
            <h3 className="text-sm font-medium text-gray-400 uppercase tracking-wide mb-2">
              Campaign Summary
            </h3>
            <div className="flex justify-between text-sm py-2 border-b border-solana-border/50">
              <span className="text-gray-500">Repository</span>
              <span className="font-mono text-white">{repo}</span>
            </div>
            <div className="flex justify-between text-sm py-2 border-b border-solana-border/50">
              <span className="text-gray-500">Reward Pool</span>
              <span className="font-mono text-solana-green">{poolSol} SOL</span>
            </div>
            <div className="flex justify-between text-sm py-2 border-b border-solana-border/50">
              <span className="text-gray-500">Deadline</span>
              <span className="text-white">{deadline}</span>
            </div>
            {user && (
              <div className="flex justify-between text-sm py-2">
                <span className="text-gray-500">Owner</span>
                <span className="text-white">@{user.github_username}</span>
              </div>
            )}
          </div>

          <div className="card border-solana-purple/20 bg-solana-purple/[0.03] text-center py-8">
            <p className="text-sm text-gray-300 mb-3">
              Sign a transaction in your wallet to fund the escrow vault
            </p>
            <p className="text-3xl font-bold text-solana-purple">{poolSol} SOL</p>
          </div>

          <p className="text-xs text-gray-500 text-center">
            The connected wallet signs the funding transaction, but future manual management stays
            limited to the GitHub account that created this campaign.
          </p>

          {error && (
            <div className="bg-red-500/10 border border-red-500/30 rounded-xl p-4 text-sm text-red-400">
              {error}
            </div>
          )}

          <div className="flex gap-4 pt-2">
            <button
              type="button"
              onClick={handleFund}
              disabled={submitting || !publicKey || !solanaReady}
              className="btn-primary flex-1 flex items-center justify-center gap-2"
            >
              {submitting
                ? 'Confirming...'
                : solanaReady
                  ? 'Fund Campaign'
                  : 'Solana Backend Required'}
              {!submitting && solanaReady && <ArrowRightIcon />}
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
