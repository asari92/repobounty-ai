import { useEffect, useState } from 'react';
import { useConnection, useWallet } from '@solana/wallet-adapter-react';
import { useWalletModal } from '@solana/wallet-adapter-react-ui';

export default function WalletButton() {
  const { connection } = useConnection();
  const { publicKey, disconnect, connecting } = useWallet();
  const { setVisible } = useWalletModal();
  const [balanceSol, setBalanceSol] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    let subscriptionId: number | null = null;

    async function loadBalance() {
      if (!publicKey) {
        setBalanceSol(null);
        return;
      }

      try {
        const lamports = await connection.getBalance(publicKey);
        if (!cancelled) {
          setBalanceSol((lamports / 1e9).toFixed(2));
        }
      } catch {
        if (!cancelled) {
          setBalanceSol(null);
        }
      }
    }

    loadBalance();

    if (publicKey) {
      subscriptionId = connection.onAccountChange(
        publicKey,
        (accountInfo) => {
          if (!cancelled) {
            setBalanceSol((accountInfo.lamports / 1e9).toFixed(2));
          }
        },
        'confirmed'
      );
    }

    return () => {
      cancelled = true;
      if (subscriptionId !== null) {
        connection.removeAccountChangeListener(subscriptionId).catch(() => {});
      }
    };
  }, [connection, publicKey]);

  if (connecting) {
    return (
      <button
        className="bg-solana-card border border-solana-border rounded-lg px-3 py-2 text-xs text-gray-400 opacity-50"
        disabled
      >
        Connecting...
      </button>
    );
  }

  if (publicKey) {
    const short = `${publicKey.toBase58().slice(0, 4)}...${publicKey.toBase58().slice(-4)}`;
    return (
      <div className="flex items-center gap-1.5">
        <span className="bg-solana-card border border-solana-border rounded-md px-2.5 py-1.5 text-[11px] text-solana-green font-mono">
          {short}
        </span>
        <span className="text-xs text-gray-400">
          {balanceSol === null ? '...' : `${balanceSol} SOL`}
        </span>
        <button
          onClick={disconnect}
          className="text-xs text-gray-600 hover:text-gray-400 transition-colors p-1"
        >
          <svg
            className="w-3.5 h-3.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    );
  }

  return (
    <button onClick={() => setVisible(true)} className="btn-primary !text-xs !py-2 !px-4">
      Connect Wallet
    </button>
  );
}
