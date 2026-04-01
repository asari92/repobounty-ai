import { Link } from 'react-router-dom';
import type { Campaign } from '../types';
import { getStateConfig, formatSOL, formatDate } from '../utils/campaign';

function ContributorAvatars({ allocations }: { allocations: Campaign['allocations'] }) {
  if (!allocations || allocations.length === 0) return null;

  const colors = [
    'bg-solana-purple',
    'bg-solana-green',
    'bg-blue-500',
    'bg-amber-500',
    'bg-rose-500',
  ];

  return (
    <div className="flex items-center -space-x-1.5">
      {allocations.slice(0, 3).map((a, i) => (
        <div
          key={a.contributor}
          className={`w-6 h-6 rounded-full ${colors[i % colors.length]} ring-2 ring-solana-card flex items-center justify-center text-[9px] font-bold text-white`}
          title={`@${a.contributor}`}
        >
          {a.contributor.slice(0, 2).toUpperCase()}
        </div>
      ))}
      {allocations.length > 3 && (
        <div className="w-6 h-6 rounded-full bg-solana-dark ring-2 ring-solana-card flex items-center justify-center text-[9px] text-gray-500">
          +{allocations.length - 3}
        </div>
      )}
    </div>
  );
}

export default function CampaignCard({ campaign }: { campaign: Campaign }) {
  const isPastDeadline = new Date(campaign.deadline) < new Date();
  const stateConfig = getStateConfig(campaign.state, isPastDeadline);

  const badgeClass =
    campaign.state === 'funded'
      ? 'badge-funded'
      : campaign.state === 'finalized'
        ? 'badge-finalized'
        : campaign.state === 'completed'
          ? 'badge-completed'
          : 'badge-created';

  const accentClass =
    campaign.state === 'funded'
      ? 'accent-funded'
      : campaign.state === 'finalized'
        ? 'accent-finalized'
        : campaign.state === 'completed'
          ? 'accent-completed'
          : 'accent-created';

  return (
    <Link to={`/campaign/${campaign.campaign_id}`} className="block group">
      <div className={`card card-hover ${accentClass} cursor-pointer h-full flex flex-col`}>
        {/* Top row: repo + badge */}
        <div className="flex items-start justify-between mb-4">
          <div className="min-w-0">
            <h3 className="font-semibold text-white text-sm truncate group-hover:text-solana-purple transition-colors duration-200">
              {campaign.repo}
            </h3>
            <p className="text-[11px] text-gray-600 font-mono mt-0.5">
              {campaign.campaign_id.slice(0, 8)}
            </p>
          </div>
          <span className={`badge ${badgeClass} flex-shrink-0 ml-3`}>{stateConfig.label}</span>
        </div>

        {/* SOL amount prominent */}
        <div className="mb-4">
          <span className="text-2xl font-bold text-solana-green tracking-tight">
            {formatSOL(campaign.pool_amount)}
          </span>
          <span className="text-xs text-gray-500 ml-1.5">SOL</span>
        </div>

        {/* Meta rows */}
        <div className="space-y-2 flex-1 text-xs">
          <div className="flex items-center justify-between">
            <span className="text-gray-600">Creator</span>
            <span className="text-gray-400 font-mono">
              {campaign.owner_github_username
                ? `@${campaign.owner_github_username}`
                : campaign.sponsor
                  ? `${campaign.sponsor.slice(0, 6)}...`
                  : '—'}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-gray-600">Deadline</span>
            <span className="text-gray-400">{formatDate(campaign.deadline)}</span>
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between mt-4 pt-3 border-t border-solana-border/50">
          <ContributorAvatars allocations={campaign.allocations ?? []} />
          <span className="text-[11px] text-gray-600 group-hover:text-solana-purple transition-all duration-200 group-hover:translate-x-0.5">
            View &rarr;
          </span>
        </div>
      </div>
    </Link>
  );
}
