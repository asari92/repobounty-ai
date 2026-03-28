import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import CampaignCard from "../components/CampaignCard";
import type { Campaign } from "../types";

export default function Home() {
  const [campaigns, setCampaigns] = useState<Campaign[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .listCampaigns()
      .then(setCampaigns)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold">
            <span className="gradient-text">Campaigns</span>
          </h1>
          <p className="text-gray-400 mt-2">
            Fund open-source contributors with AI-powered reward allocation on
            Solana
          </p>
        </div>
        <Link to="/create" className="btn-primary">
          + New Campaign
        </Link>
      </div>

      {loading && (
        <div className="text-center py-20">
          <div className="inline-block w-8 h-8 border-2 border-solana-purple border-t-transparent rounded-full animate-spin" />
          <p className="text-gray-400 mt-4">Loading campaigns...</p>
        </div>
      )}

      {error && (
        <div className="card border-red-500/30 bg-red-500/10 text-center py-12">
          <p className="text-red-400">{error}</p>
          <button
            onClick={() => window.location.reload()}
            className="btn-secondary text-sm mt-4"
          >
            Retry
          </button>
        </div>
      )}

      {!loading && !error && campaigns.length === 0 && (
        <div className="card text-center py-16">
          <div className="text-4xl mb-4">{"{ }"}</div>
          <h3 className="text-xl font-semibold mb-2">No campaigns yet</h3>
          <p className="text-gray-400 mb-6">
            Create your first campaign to fund open-source contributors
          </p>
          <Link to="/create" className="btn-primary">
            Create Campaign
          </Link>
        </div>
      )}

      {!loading && !error && campaigns.length > 0 && (
        <div className="grid gap-4 md:grid-cols-2">
          {campaigns.map((c) => (
            <CampaignCard key={c.campaign_id} campaign={c} />
          ))}
        </div>
      )}
    </div>
  );
}
