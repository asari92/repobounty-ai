import { useWallet } from "@solana/wallet-adapter-react";
import { useWalletModal } from "@solana/wallet-adapter-react-ui";

export default function WalletButton() {
  const { publicKey, disconnect, connecting } = useWallet();
  const { setVisible } = useWalletModal();

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
        <span className="text-sm text-solana-green font-mono">{short}</span>
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
