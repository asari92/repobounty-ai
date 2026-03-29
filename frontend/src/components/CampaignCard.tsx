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
  const isCompleted = campaign.state === "completed";
  const isPastDeadline = new Date(campaign.deadline) < new Date();

  const getStateColor = () => {
    switch (campaign.state) {
      case "completed":
        return "bg-solana-green/20 text-solana-green";
      case "finalized":
        return "bg-solana-green/20 text-solana-green";
      case "funded":
        return "bg-blue-500/20 text-blue-400";
      case "created":
        return isPastDeadline ? "bg-yellow-500/20 text-yellow-400" : "bg-solana-purple/20 text-solana-purple";
      default:
        return "bg-solana-purple/20 text-solana-purple";
    }
  };

  const getStateLabel = () => {
    switch (campaign.state) {
      case "completed":
        return "Completed";
      case "finalized":
        return "Finalized";
      case "funded":
        return "Funded";
      case "created":
        return isPastDeadline ? "Ready to Fund" : "Created";
      default:
        return "Active";
    }
  };

  return (
    <Link to={`/campaign/${campaign.campaign_id}`} className="block">
      <div className="card hover:border-solana-purple transition-colors cursor-pointer">
        <div className="flex items-start justify-between mb-4">
          <div>
            <h3 className="font-semibold text-lg">{campaign.repo}</h3>
            <p className="text-sm text-gray-400 mt-1">
              ID: {campaign.campaign_id}
            </p>
            {campaign.sponsor && (
              <p className="text-xs text-gray-500 mt-0.5">
                by {campaign.sponsor.slice(0, 8)}...
              </p>
            )}
          </div>
          <span
            className={`text-xs font-semibold px-3 py-1 rounded-full ${getStateColor()}`}
          >
            {getStateLabel()}
          </span>
        </div>

        <div className="grid grid-cols-3 gap-4 text-sm">
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
          {(isCompleted || isFinalized) && campaign.allocations.length > 0 && (
            <div>
              <span className="text-gray-400">Claimed</span>
              <p className="font-semibold">
                {formatSOL(campaign.total_claimed)} / {formatSOL(campaign.pool_amount)}
              </p>
            </div>
          )}
        </div>

        {isFinalized && campaign.allocations.length > 0 && (
          <div className="mt-4 pt-4 border-t border-solana-border">
            <div className="flex items-center gap-2 flex-wrap">
              {campaign.allocations.slice(0, 3).map((a) => (
                <span
                  key={a.contributor}
                  className={`text-xs px-2 py-1 rounded ${
                    a.claimed ? "bg-solana-green/20 text-solana-green" : "bg-solana-dark"
                  }`}
                >
                  @{a.contributor} ({(a.percentage / 100).toFixed(0)}%)
                  {a.claimed && " ✓"}
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
