import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../api/client';
import CampaignCard from '../components/CampaignCard';
import { useAuth } from '../hooks/useAuth';
import type { Campaign } from '../types';

function SearchIcon() {
  return (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z"
      />
    </svg>
  );
}

export default function Home() {
  const { user } = useAuth();
  const [campaigns, setCampaigns] = useState<Campaign[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [view, setView] = useState<'all' | 'mine'>('all');
  const [search, setSearch] = useState('');

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
    return () => {
      cancelled = true;
    };
  }, []);

  const visibleCampaigns = useMemo(() => {
    let filtered =
      view === 'mine' && user
        ? campaigns.filter((campaign) => campaign.owner_github_username === user.github_username)
        : view === 'mine'
          ? []
          : campaigns;

    if (search.trim()) {
      const q = search.toLowerCase();
      filtered = filtered.filter(
        (c) =>
          c.repo.toLowerCase().includes(q) ||
          c.campaign_id.toLowerCase().includes(q) ||
          c.owner_github_username?.toLowerCase().includes(q)
      );
    }

    return filtered;
  }, [campaigns, user, view, search]);

  return (
    <div>
      {/* Hero section */}
      <div className="mb-10">
        <h1 className="text-5xl font-bold tracking-tight mb-3">
          <span className="gradient-text">Campaigns</span>
        </h1>
        <p className="text-gray-400 text-lg max-w-xl">
          Fund open-source contributors with AI-powered reward allocation on Solana
        </p>
      </div>

      {/* Controls bar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 mb-8">
        <div className="inline-flex rounded-xl border border-solana-border bg-solana-card p-1">
          <button
            onClick={() => setView('all')}
            className={`px-5 py-2 text-sm font-medium rounded-lg transition-all ${
              view === 'all'
                ? 'bg-solana-purple text-white shadow-lg shadow-solana-purple/25'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            All Campaigns
          </button>
          <button
            onClick={() => setView('mine')}
            className={`px-5 py-2 text-sm font-medium rounded-lg transition-all ${
              view === 'mine'
                ? 'bg-solana-purple text-white shadow-lg shadow-solana-purple/25'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            My Campaigns
          </button>
        </div>

        <div className="flex items-center gap-3">
          <div className="relative">
            <div className="absolute inset-y-0 left-3 flex items-center pointer-events-none text-gray-500">
              <SearchIcon />
            </div>
            <input
              type="text"
              placeholder="Search repositories..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="bg-solana-card border border-solana-border rounded-xl pl-10 pr-4 py-2.5 text-sm text-white placeholder-gray-500 focus:outline-none focus:border-solana-purple/60 focus:ring-1 focus:ring-solana-purple/30 transition-all w-64"
            />
          </div>
          <Link to="/create" className="btn-primary flex items-center gap-2 text-sm !py-2.5">
            <span className="text-lg leading-none">+</span> New Campaign
          </Link>
        </div>
      </div>

      {loading && (
        <div className="text-center py-24">
          <div className="inline-block w-10 h-10 border-2 border-solana-purple border-t-transparent rounded-full animate-spin" />
          <p className="text-gray-500 mt-4 text-sm">Loading campaigns...</p>
        </div>
      )}

      {error && (
        <div className="card border-red-500/30 bg-red-500/10 text-center py-12">
          <p className="text-red-400">{error}</p>
          <button onClick={() => window.location.reload()} className="btn-secondary text-sm mt-4">
            Retry
          </button>
        </div>
      )}

      {!loading && !error && view === 'mine' && !user && (
        <div className="card text-center py-20">
          <div className="w-16 h-16 rounded-2xl bg-solana-purple/10 flex items-center justify-center mx-auto mb-4">
            <svg
              className="w-8 h-8 text-solana-purple"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={1.5}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z"
              />
            </svg>
          </div>
          <h3 className="text-xl font-semibold mb-2">Log in with GitHub</h3>
          <p className="text-gray-500 text-sm">
            Sign in to view campaigns created from your RepoBounty account.
          </p>
        </div>
      )}

      {!loading && !error && visibleCampaigns.length === 0 && !(view === 'mine' && !user) && (
        <div className="card text-center py-20">
          <div className="w-16 h-16 rounded-2xl bg-solana-card border border-solana-border flex items-center justify-center mx-auto mb-4">
            <span className="text-2xl text-gray-500">{'{ }'}</span>
          </div>
          <h3 className="text-xl font-semibold mb-2">
            {view === 'mine' ? 'No campaigns for this account' : 'No campaigns yet'}
          </h3>
          <p className="text-gray-500 text-sm mb-6">
            {view === 'mine'
              ? 'Create a campaign while signed in with GitHub to see it here.'
              : 'Create your first campaign to fund open-source contributors'}
          </p>
          <Link to="/create" className="btn-primary inline-block">
            Create Campaign
          </Link>
        </div>
      )}

      {!loading && !error && visibleCampaigns.length > 0 && (
        <>
          <p className="text-xs text-gray-500 mb-4">
            Showing {visibleCampaigns.length} campaign
            {visibleCampaigns.length === 1 ? '' : 's'}
            {view === 'mine' && user ? ' you created' : ''}
          </p>
          <div className="grid gap-5 md:grid-cols-2 lg:grid-cols-3">
            {visibleCampaigns.map((c) => (
              <CampaignCard key={c.campaign_id} campaign={c} />
            ))}
          </div>
        </>
      )}
    </div>
  );
}
