import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useConnection, useWallet } from '@solana/wallet-adapter-react';
import { useWalletModal } from '@solana/wallet-adapter-react-ui';
import { Transaction } from '@solana/web3.js';
import bs58 from 'bs58';
import { api } from '../api/client';

const MIN_CAMPAIGN_POOL_SOL = 0.5;
const MIN_CAMPAIGN_POOL_LAMPORTS = 500_000_000;

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

class ConfirmationTimeoutError extends Error {}

function isRejectedWalletError(err: unknown): boolean {
  const message = err instanceof Error ? err.message : String(err);
  const lower = message.toLowerCase();
  return lower.includes('rejected') || lower.includes('user rejected');
}

interface PendingCreate {
  repo: string;
  campaignId: string;
  sponsorWallet: string;
  poolLamports: number;
  deadlineRFC3339: string;
  txSignature: string;
}

type RpcTransactionState = 'pending' | 'confirmed' | 'failed';

export default function CreateCampaign() {
  const { publicKey, signTransaction } = useWallet();
  const { connection } = useConnection();
  const { setVisible } = useWalletModal();
  const navigate = useNavigate();

  const [repo, setRepo] = useState('');
  const [poolSol, setPoolSol] = useState('');
  const [deadline, setDeadline] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [pendingCreate, setPendingCreate] = useState<PendingCreate | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [solanaReady, setSolanaReady] = useState(true);

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
    repoName: string,
    sponsorWallet: string,
    poolLamports: number,
    deadlineRFC3339: string,
    txSignature: string
  ) {
    for (let attempt = 0; attempt < 12; attempt += 1) {
      try {
        await api.createCampaignConfirm(campaignId, {
          repo: repoName,
          pool_amount: poolLamports,
          deadline: deadlineRFC3339,
          sponsor_wallet: sponsorWallet,
          tx_signature: txSignature,
        });
        return;
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : 'Failed to confirm campaign creation';
        if (
          message.includes('campaign transaction is not confirmed on-chain yet') ||
          message.includes('campaign not found')
        ) {
          await delay(1500);
          continue;
        }
        throw (err instanceof Error ? err : new Error(message));
      }
    }

    throw new ConfirmationTimeoutError(
      'Campaign transaction was sent, but backend confirmation is still pending. You can retry confirmation with the same transaction.'
    );
  }

  async function lookupTransactionState(txSignature: string): Promise<RpcTransactionState> {
    try {
      const { value } = await connection.getSignatureStatuses([txSignature], {
        searchTransactionHistory: true,
      });
      const status = value[0];

      if (status?.err) {
        return 'failed';
      }
      if (
        status &&
        (status.confirmationStatus === 'confirmed' ||
          status.confirmationStatus === 'finalized' ||
          status.confirmations === null)
      ) {
        return 'confirmed';
      }
    } catch {
      return 'pending';
    }

    return 'pending';
  }

  async function finalizePendingCreate(pending: PendingCreate) {
    setError(null);
    setStatusMessage('Transaction sent. Waiting for on-chain confirmation...');
    try {
      await confirmCreatedCampaign(
        pending.campaignId,
        pending.repo,
        pending.sponsorWallet,
        pending.poolLamports,
        pending.deadlineRFC3339,
        pending.txSignature
      );
      setPendingCreate(null);
      setStatusMessage(null);
      navigate(`/campaign/${pending.campaignId}`);
    } catch (err: unknown) {
      const txState = await lookupTransactionState(pending.txSignature);
      if (txState === 'failed') {
        setPendingCreate(null);
        setStatusMessage(null);
        setError('Transaction failed on-chain. Create a new transaction to try again.');
        return;
      }
      if (err instanceof ConfirmationTimeoutError) {
        setError(err.message);
        setStatusMessage(
          txState === 'confirmed'
            ? 'Transaction confirmed on-chain. Backend confirmation is still pending.'
            : 'Transaction sent. Waiting for on-chain confirmation...'
        );
        return;
      }
      setError(
        err instanceof Error
          ? `${err.message}. You can retry confirmation with the same transaction.`
          : 'Failed to finalize campaign creation. You can retry confirmation with the same transaction.'
      );
      setStatusMessage(
        txState === 'confirmed'
          ? 'Transaction confirmed on-chain. Backend confirmation is still pending.'
          : 'Transaction sent. Waiting for on-chain confirmation...'
      );
    }
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setStatusMessage(null);

    if (pendingCreate) {
      setSubmitting(true);
      try {
        await finalizePendingCreate(pendingCreate);
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : 'Failed to confirm campaign creation');
      } finally {
        setSubmitting(false);
      }
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

    if (!solanaReady) {
      setError('Campaign creation is unavailable until the backend is connected to Solana.');
      return;
    }
    if (!publicKey) {
      setError('Connect a wallet to create a campaign.');
      setVisible(true);
      return;
    }
    if (!signTransaction) {
      setError('This wallet does not support transaction signing.');
      return;
    }

    setSubmitting(true);
    try {
      const result = await api.createCampaign({
        repo,
        pool_amount: poolLamports,
        deadline: deadlineRFC3339,
        sponsor_wallet: publicKey.toBase58(),
      });

      if (!result.campaign_id?.trim()) {
        throw new Error('Campaign ID was not returned by the backend.');
      }
      if (!result.unsigned_tx?.trim()) {
        throw new Error('Create transaction was not returned by the backend.');
      }

      const txBytes = bs58.decode(result.unsigned_tx);
      const transaction = Transaction.from(txBytes);

      let signedTransaction;
      try {
        signedTransaction = await signTransaction(transaction);
      } catch (err: unknown) {
        if (isRejectedWalletError(err)) {
          throw new Error('Transaction rejected by wallet.');
        }
        throw new Error('Failed to sign transaction.');
      }

      let txSignature: string;
      try {
        txSignature = await connection.sendRawTransaction(signedTransaction.serialize());
      } catch (err: unknown) {
        if (isRejectedWalletError(err)) {
          throw new Error('Transaction rejected by wallet.');
        }
        throw new Error('Failed to send transaction.');
      }

      const pending: PendingCreate = {
        repo,
        campaignId: result.campaign_id,
        sponsorWallet: publicKey.toBase58(),
        poolLamports,
        deadlineRFC3339,
        txSignature,
      };
      setPendingCreate(pending);
      await finalizePendingCreate(pending);
    } catch (err: unknown) {
      if (!(err instanceof ConfirmationTimeoutError)) {
        setPendingCreate(null);
        setStatusMessage(null);
      }
      setError(err instanceof Error ? err.message : 'Failed to create campaign');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="max-w-lg mx-auto">
      <div className="mb-6 animate-fade-in-up">
        <h1 className="text-2xl font-bold tracking-tight mb-1">
          <span className="gradient-text">Create Campaign</span>
        </h1>
        <p className="text-xs text-gray-500">
          Fund a GitHub repo and let AI allocate rewards to contributors
        </p>
        <div className="gradient-line mt-3" />
      </div>

      {!solanaReady && (
        <div className="bg-yellow-500/5 border border-yellow-500/20 rounded-lg p-3 text-xs text-yellow-200 mb-5">
          Backend is not connected to Solana. Creating campaigns is disabled.
        </div>
      )}

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
              disabled={submitting || pendingCreate !== null}
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
                disabled={submitting || pendingCreate !== null}
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
                step="60"
                className="input"
                disabled={submitting || pendingCreate !== null}
                required
              />
            </div>
          </div>

          <div className="card !p-4 !bg-solana-green/[0.03] !border-solana-green/15 text-xs text-gray-400 space-y-1.5">
            <p>
              <span className="text-solana-green font-medium">AI analysis</span> - commits and PRs
              are scored automatically
            </p>
            <p>
              <span className="text-solana-green font-medium">Escrow</span> - funds held in Solana
              smart contract until deadline
            </p>
            <p>
              <span className="text-solana-green font-medium">Minimum pool</span> -{' '}
              {MIN_CAMPAIGN_POOL_SOL} SOL
            </p>
            <p>
              <span className="text-solana-green font-medium">Merged code only</span> - only main
              branch contributions qualify
            </p>
          </div>

          {error && (
            <div className="bg-red-500/5 border border-red-500/15 rounded-lg p-3 text-xs text-red-400">
              {error}
            </div>
          )}

          {statusMessage && !error && (
            <div className="bg-blue-500/5 border border-blue-500/20 rounded-lg p-3 text-xs text-blue-200">
              {statusMessage}
            </div>
          )}

          <div className="flex gap-3 pt-1">
            <button
              type="submit"
              disabled={submitting || !solanaReady}
              className="btn-primary flex-1"
            >
              {submitting
                ? pendingCreate
                  ? 'Confirming...'
                  : 'Creating...'
                : pendingCreate
                  ? 'Retry confirmation'
                  : !publicKey
                    ? 'Connect Wallet'
                    : solanaReady
                      ? 'Create Campaign'
                      : 'Solana Required'}
            </button>
            <button
              type="button"
              onClick={() => navigate('/')}
              className="btn-secondary"
              disabled={submitting}
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
