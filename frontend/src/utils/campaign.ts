import type { Campaign } from "../types";

export function getStateConfig(state: Campaign["state"], isPastDeadline = false) {
  switch (state) {
    case "completed":
      return { label: "Completed", classes: "bg-solana-green/20 text-solana-green" };
    case "finalized":
      return { label: "Finalized", classes: "bg-solana-green/20 text-solana-green" };
    case "funded":
      return { label: "Funded", classes: "bg-blue-500/20 text-blue-400" };
    case "created":
      return {
        label: isPastDeadline ? "Ready to Fund" : "Created",
        classes: isPastDeadline
          ? "bg-yellow-500/20 text-yellow-400"
          : "bg-solana-purple/20 text-solana-purple",
      };
    default:
      return { label: state, classes: "bg-solana-purple/20 text-solana-purple" };
  }
}

export function formatSOL(lamports: number, decimals = 4): string {
  return (lamports / 1e9).toFixed(decimals);
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString("en-US", {
    dateStyle: "medium",
    timeStyle: "short",
  });
}
