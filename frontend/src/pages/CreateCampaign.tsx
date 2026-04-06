import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useWallet } from '@solana/wallet-adapter-react';
import { useConnection } from '@solana/wallet-adapter-react';
import { useWalletModal } from '@solana/wallet-adapter-react-ui';
import { Transaction } from '@solana/web3.js';
import bs58 from 'bs58';
import { api } from '../api/client';

const MIN_CAMPAIGN_POOL_SOL = 0.5;
const MIN_CAMPAIGN_POOL_LAMPORTS = 500_000_000;

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

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

export default function CreateCampaign() {
  const { publicKey, signMessage, signTransaction } = useWallet();
  const { connection } = useConnection();
  const { setVisible } = useWalletModal();
  const navigate = useNavigate();

  const [step, setStep] = useState<1 | 2>(1);
  const [repo, setRepo] = useState('');
  const [poolSol, setPoolSol] = useState('');
  const [deadline, setDeadline] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [createdId, setCreatedId] = useState<string | null>(null);
  const [preparedTx, setPreparedTx] = useState<string | null>(null);
  const [preparedSponsorWallet, setPreparedSponsorWallet] = useState<string | null>(null);
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

  async function confirmCreatedCampaign(
    campaignId: string,
    sponsorWallet: string,
    poolLamports: number,
    deadlineRFC3339: string,
    txSignature: string
  ) {
    let lastError: Error | null = null;

    for (let attempt = 0; attempt < 12; attempt += 1) {
      try {
        await api.createCampaignConfirm(campaignId, {
          repo,
          pool_amount: poolLamports,
          deadline: deadlineRFC3339,
          sponsor_wallet: sponsorWallet,
          tx_signature: txSignature,
        });
        return;
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : 'Failed to confirm campaign creation';
        lastError = err instanceof Error ? err : new Error(message);
        if (
          message.includes('campaign transaction is not confirmed on-chain yet') ||
          message.includes('campaign not found')
        ) {
          await delay(1500);
          continue;
        }
        throw lastError;
      }
    }

    throw (
      lastError ??
      new Error('Campaign transaction was sent, but on-chain confirmation took too long.')
    );
  }

  async function submitPreparedCampaign(
    campaignId: string,
    txBase58: string,
    sponsorWallet: string,
    poolLamports: number,
    deadlineRFC3339: string
  ) {
    if (!publicKey) {
      setVisible(true);
      return;
    }
    if (!signTransaction) {
      throw new Error('This wallet does not support transaction signing.');
    }
    if (!solanaReady) {
      throw new Error('Campaign creation is unavailable until the backend is connected to Solana.');
    }
    if (publicKey.toBase58() !== sponsorWallet) {
      throw new Error('Reconnect the same sponsor wallet that prepared this campaign transaction.');
    }

    const txBytes = bs58.decode(txBase58);
    const transaction = Transaction.from(txBytes);

    const signedTransaction = await signTransaction(transaction);
    const signature = await connection.sendRawTransaction(signedTransaction.serialize());
    await confirmCreatedCampaign(
      campaignId,
      sponsorWallet,
      poolLamports,
      deadlineRFC3339,
      signature
    );

    navigate(`/campaign/${campaignId}`);
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    if (!publicKey) {
      setVisible(true);
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
    if (poolLamports < MIN_CAMPAIGN_POOL_LAMPORTS) {
      setError(`Pool amount must be at least ${MIN_CAMPAIGN_POOL_SOL} SOL`);
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
      if (!result.campaign_id?.trim()) {
        throw new Error('Campaign ID was not returned by the backend.');
      }
      if (!result.unsigned_tx?.trim()) {
        throw new Error('Create transaction was not returned by the backend.');
      }
      setCreatedId(result.campaign_id);
      setPreparedTx(result.unsigned_tx);
      setPreparedSponsorWallet(publicKey.toBase58());
      setStep(2);
      await submitPreparedCampaign(
        result.campaign_id,
        result.unsigned_tx,
        publicKey.toBase58(),
        poolLamports,
        deadlineRFC3339
      );
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to create campaign');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleFund() {
    if (!publicKey || !createdId || !preparedTx || !preparedSponsorWallet) return;
    setError(null);

    if (!solanaReady) {
      setError('Campaign creation is unavailable until the backend is connected to Solana.');
      return;
    }
    if (publicKey.toBase58() !== preparedSponsorWallet) {
      setError('Reconnect the same sponsor wallet that prepared this campaign transaction.');
      return;
    }

    setSubmitting(true);

    try {
      await submitPreparedCampaign(
        createdId,
        preparedTx,
        preparedSponsorWallet,
        Math.round(parseFloat(poolSol) * 1e9),
        toStableRFC3339(deadline) ?? deadline
      );
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

  function handleBack() {
    if (step === 2) {
      setStep(1);
      setPreparedTx(null);
      setPreparedSponsorWallet(null);
      setCreatedId(null);
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
          Approve
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
                  placeholder="0.5"
                  step="0.01"
                  min={String(MIN_CAMPAIGN_POOL_SOL)}
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
              <p><span className="text-solana-green font-medium">Minimum pool</span> — {MIN_CAMPAIGN_POOL_SOL} SOL</p>
              <p><span className="text-solana-green font-medium">Merged code only</span> — only main branch contributions qualify</p>
            </div>

            {error && (
              <div className="bg-red-500/5 border border-red-500/15 rounded-lg p-3 text-xs text-red-400">
                {error}
              </div>
            )}

            <div className="flex gap-3 pt-1">
              {!publicKey ? (
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
                  {submitting ? 'Preparing...' : 'Prepare Deposit →'}
                </button>
              )}
              <button type="button" onClick={handleBack} className="btn-secondary">
                Cancel
              </button>
            </div>
          </form>
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
              <div className="flex justify-between py-1.5">
                <span className="text-gray-500">Sponsor wallet</span>
                <span className="font-mono text-white text-xs">{preparedSponsorWallet ?? 'N/A'}</span>
              </div>
            </div>
          </div>

          <div className="stat-block text-center py-6">
            <p className="text-xs text-gray-500 mb-2">Sign the sponsor transaction to create and fund the campaign</p>
            <p className="text-3xl font-bold text-solana-purple">{poolSol} SOL</p>
          </div>

          <p className="text-[10px] text-gray-600 text-center">
            The campaign does not exist yet. It will appear only after this wallet signs and the
            on-chain transaction is confirmed.
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
              disabled={submitting || !publicKey || !solanaReady || !preparedTx}
              className="btn-primary flex-1"
            >
              {submitting
                ? 'Confirming...'
                : solanaReady
                  ? 'Create Campaign →'
                  : 'Solana Required'}
            </button>
            <button
              type="button"
              onClick={handleBack}
              className="btn-secondary"
              disabled={submitting}
            >
              Back
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
