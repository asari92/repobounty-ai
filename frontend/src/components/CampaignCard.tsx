import { Link } from "react-router-dom";
import type { Campaign } from "../types";

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString("en-US", {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function formatSOL(lamports: number): string {
  return (lamports / 1e9).toFixed(2);
}

export default function CampaignCard({ campaign }: { campaign: Campaign }) {
  const isFinalized = campaign.state === "finalized";
  const isPastDeadline = new Date(campaign.deadline) < new Date();

  return (
    <Link to={`/campaign/${campaign.campaign_id}`} className="block">
      <div className="card hover:border-solana-purple transition-colors cursor-pointer">
        <div className="flex items-start justify-between mb-4">
          <div>
            <h3 className="font-semibold text-lg">{campaign.repo}</h3>
            <p className="text-sm text-gray-400 mt-1">
              ID: {campaign.campaign_id}
            </p>
          </div>
          <span
            className={`text-xs font-semibold px-3 py-1 rounded-full ${
              isFinalized
                ? "bg-solana-green/20 text-solana-green"
                : isPastDeadline
                  ? "bg-yellow-500/20 text-yellow-400"
                  : "bg-solana-purple/20 text-solana-purple"
            }`}
          >
            {isFinalized
              ? "Finalized"
              : isPastDeadline
                ? "Ready to Finalize"
                : "Active"}
          </span>
        </div>

        <div className="grid grid-cols-2 gap-4 text-sm">
          <div>
            <span className="text-gray-400">Pool</span>
            <p className="font-semibold text-solana-green">
              {formatSOL(campaign.pool_amount)} SOL
            </p>
          </div>
          <div>
            <span className="text-gray-400">Deadline</span>
            <p className="font-semibold">{formatDate(campaign.deadline)}</p>
          </div>
        </div>

        {isFinalized && campaign.allocations.length > 0 && (
          <div className="mt-4 pt-4 border-t border-solana-border">
            <div className="flex items-center gap-2 flex-wrap">
              {campaign.allocations.slice(0, 3).map((a) => (
                <span
                  key={a.contributor}
                  className="text-xs bg-solana-dark px-2 py-1 rounded"
                >
                  @{a.contributor} ({(a.percentage / 100).toFixed(0)}%)
                </span>
              ))}
              {campaign.allocations.length > 3 && (
                <span className="text-xs text-gray-500">
                  +{campaign.allocations.length - 3} more
                </span>
              )}
            </div>
          </div>
        )}
      </div>
    </Link>
  );
}
