import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api/client";
import CampaignCard from "../components/CampaignCard";
import { useAuth } from "../hooks/useAuth";
import type { Campaign } from "../types";

export default function Home() {
  const { user } = useAuth();
  const [campaigns, setCampaigns] = useState<Campaign[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [view, setView] = useState<"all" | "mine">("all");

  useEffect(() => {
    let cancelled = false;
    api
      .listCampaigns()
      .then((data) => {
        if (!cancelled) setCampaigns(data);
      })
      .catch((e) => {
        if (!cancelled) setError(e.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
  }, []);

  const visibleCampaigns = useMemo(
    () =>
      view === "mine" && user
        ? campaigns.filter((campaign) => campaign.owner_github_username === user.github_username)
        : view === "mine"
          ? []
          : campaigns,
    [campaigns, user, view]
  );

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

      <div className="flex flex-wrap items-center justify-between gap-4 mb-6">
        <div className="inline-flex rounded-xl border border-solana-border bg-solana-dark/70 p-1">
          <button
            onClick={() => setView("all")}
            className={`px-4 py-2 text-sm rounded-lg transition-colors ${
              view === "all"
                ? "bg-solana-purple text-white"
                : "text-gray-400 hover:text-white"
            }`}
          >
            All Campaigns
          </button>
          <button
            onClick={() => setView("mine")}
            className={`px-4 py-2 text-sm rounded-lg transition-colors ${
              view === "mine"
                ? "bg-solana-purple text-white"
                : "text-gray-400 hover:text-white"
            }`}
          >
            My Campaigns
          </button>
        </div>

        <p className="text-sm text-gray-400">
          Showing {visibleCampaigns.length} campaign
          {visibleCampaigns.length === 1 ? "" : "s"}
          {view === "mine" && user ? " you created" : ""}
        </p>
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

      {!loading && !error && view === "mine" && !user && (
        <div className="card text-center py-16">
          <h3 className="text-xl font-semibold mb-2">
            Log in with GitHub
          </h3>
          <p className="text-gray-400">
            Sign in to view campaigns created from your RepoBounty account.
          </p>
        </div>
      )}

      {!loading && !error && visibleCampaigns.length === 0 && !(view === "mine" && !user) && (
        <div className="card text-center py-16">
          <div className="text-4xl mb-4">{"{ }"}</div>
          <h3 className="text-xl font-semibold mb-2">
            {view === "mine" ? "No campaigns for this account" : "No campaigns yet"}
          </h3>
          <p className="text-gray-400 mb-6">
            {view === "mine"
              ? "Create a campaign while signed in with GitHub to see it here."
              : "Create your first campaign to fund open-source contributors"}
          </p>
          <Link to="/create" className="btn-primary">
            Create Campaign
          </Link>
        </div>
      )}

      {!loading && !error && visibleCampaigns.length > 0 && (
        <div className="grid gap-4 md:grid-cols-2">
          {visibleCampaigns.map((c) => (
            <CampaignCard key={c.campaign_id} campaign={c} />
          ))}
        </div>
      )}
    </div>
  );
}
