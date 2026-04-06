import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useWallet } from '@solana/wallet-adapter-react';
import { api } from '../api/client';
import CampaignCard from '../components/CampaignCard';
import { useAuth } from '../hooks/useAuth';
import type { Campaign } from '../types';

export default function Home() {
  const { user } = useAuth();
  const { publicKey } = useWallet();
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
      view === 'mine' && (user || publicKey)
        ? campaigns.filter((campaign) => {
            const walletAddress = publicKey?.toBase58() || user?.wallet_address;
            const isSponsor = walletAddress && 
              (campaign.sponsor === walletAddress || 
               campaign.authority === walletAddress);
            const isCreator = user?.github_username && 
              campaign.owner_github_username === user.github_username;
            const isContributor = user?.github_username && 
              campaign.allocations?.some(a => a.contributor === user.github_username);
            
            return isSponsor || isCreator || isContributor;
          })
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
      {/* Hero — compact, left-aligned */}
      <div className="mb-8 animate-fade-in-up">
        <div className="flex items-end justify-between gap-4">
          <div>
            <h1 className="text-3xl font-bold tracking-tight text-white mb-1">
              Campaigns
            </h1>
            <p className="text-sm text-gray-500">
              AI-powered reward allocation for open-source contributors on Solana
            </p>
          </div>
          <Link to="/create" className="btn-primary hidden sm:flex items-center gap-1.5 flex-shrink-0">
            <span className="text-base leading-none">+</span> New
          </Link>
        </div>
        <div className="gradient-line mt-4" />
      </div>

      {/* Controls */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 mb-6 animate-fade-in" style={{ animationDelay: '80ms' }}>
        <div className="flex items-center gap-1 bg-solana-card rounded-lg border border-solana-border p-0.5">
          <button
            onClick={() => setView('all')}
            className={`px-4 py-1.5 text-xs font-medium rounded-md transition-all duration-200 ${
              view === 'all'
                ? 'bg-solana-purple text-white'
                : 'text-gray-500 hover:text-gray-300'
            }`}
          >
            All
          </button>
          <button
            onClick={() => setView('mine')}
            className={`px-4 py-1.5 text-xs font-medium rounded-md transition-all duration-200 ${
              view === 'mine'
                ? 'bg-solana-purple text-white'
                : 'text-gray-500 hover:text-gray-300'
            }`}
          >
            Mine
          </button>
        </div>

        <input
          type="text"
          placeholder="Search..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="input !py-1.5 !text-xs max-w-[200px]"
        />
      </div>

      {loading && (
        <div className="text-center py-20">
          <div className="inline-block w-8 h-8 border-2 border-solana-purple border-t-transparent rounded-full animate-spin" />
          <p className="text-gray-600 mt-3 text-xs">Loading campaigns...</p>
        </div>
      )}

      {error && (
        <div className="card border-red-500/20 bg-red-500/5 text-center py-10">
          <p className="text-red-400 text-sm">{error}</p>
          <button onClick={() => window.location.reload()} className="btn-secondary text-xs mt-3">
            Retry
          </button>
        </div>
      )}

      {!loading && !error && view === 'mine' && !user && !publicKey && (
        <div className="card text-center py-16">
          <p className="text-gray-500 text-sm mb-1">Connect GitHub or wallet to see your campaigns.</p>
        </div>
      )}

      {!loading && !error && visibleCampaigns.length === 0 && !(view === 'mine' && !user && !publicKey) && (
        <div className="card text-center py-16">
          <p className="text-gray-500 text-sm mb-4">
            {view === 'mine' ? 'No campaigns found for your connected accounts.' : 'No campaigns yet.'}
          </p>
          <Link to="/create" className="btn-primary text-xs">
            Create Campaign
          </Link>
        </div>
      )}

      {!loading && !error && visibleCampaigns.length > 0 && (
        <>
          <p className="text-[11px] text-gray-600 mb-3">
            {visibleCampaigns.length} campaign{visibleCampaigns.length === 1 ? '' : 's'}
            {view === 'mine' && (user || publicKey) ? ' related to your accounts' : ''}
          </p>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {visibleCampaigns.map((c, i) => (
              <div
                key={c.campaign_id}
                className="animate-fade-in-up"
                style={{ animationDelay: `${Math.min(i * 60, 400)}ms` }}
              >
                <CampaignCard campaign={c} />
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
