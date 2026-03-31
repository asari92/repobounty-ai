import { useWallet } from '@solana/wallet-adapter-react';
import { useWalletModal } from '@solana/wallet-adapter-react-ui';

export default function WalletButton() {
  const { publicKey, disconnect, connecting } = useWallet();
  const { setVisible } = useWalletModal();

  if (connecting) {
    return (
      <button
        className="bg-solana-card border border-solana-border rounded-xl px-4 py-2.5 text-sm text-gray-400 opacity-50"
        disabled
      >
        Connecting...
      </button>
    );
  }

  if (publicKey) {
    const short = `${publicKey.toBase58().slice(0, 4)}...${publicKey.toBase58().slice(-4)}`;
    return (
      <div className="flex items-center gap-2">
        <span className="bg-solana-card border border-solana-border rounded-xl px-3 py-2 text-xs text-solana-green font-mono">
          {short}
        </span>
        <button
          onClick={disconnect}
          className="text-xs text-gray-500 hover:text-gray-300 transition-colors"
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    );
  }

  return (
    <button onClick={() => setVisible(true)} className="btn-primary text-sm !py-2.5 !px-5">
      Connect Wallet
    </button>
  );
}
