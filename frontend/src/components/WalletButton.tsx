import { useEffect, useState } from "react";
import { useConnection, useWallet } from "@solana/wallet-adapter-react";
import { useWalletModal } from "@solana/wallet-adapter-react-ui";

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
        "confirmed"
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
      <button className="btn-secondary text-sm opacity-50" disabled>
        Connecting...
      </button>
    );
  }

  if (publicKey) {
    const short = `${publicKey.toBase58().slice(0, 4)}...${publicKey.toBase58().slice(-4)}`;
    return (
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-2">
          <span className="text-sm text-solana-green font-mono">{short}</span>
          <span className="text-xs text-gray-400">
            {balanceSol === null ? "..." : `${balanceSol} SOL`}
          </span>
        </div>
        <button
          onClick={disconnect}
          className="text-sm text-gray-400 hover:text-white transition-colors"
        >
          Disconnect
        </button>
      </div>
    );
  }

  return (
    <button onClick={() => setVisible(true)} className="btn-primary text-sm">
      Connect Wallet
    </button>
  );
}
