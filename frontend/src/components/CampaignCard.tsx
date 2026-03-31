import { Link } from 'react-router-dom';
import type { Campaign } from '../types';
import { getStateConfig, formatSOL, formatDate } from '../utils/campaign';

function RepoIcon() {
  return (
    <svg
      className="w-5 h-5 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={1.5}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M3.75 9.776c.112-.017.227-.026.344-.026h15.812c.117 0 .232.009.344.026m-16.5 0a2.25 2.25 0 00-1.883 2.542l.857 6a2.25 2.25 0 002.227 1.932H19.05a2.25 2.25 0 002.227-1.932l.857-6a2.25 2.25 0 00-1.883-2.542m-16.5 0V6A2.25 2.25 0 016 3.75h3.879a1.5 1.5 0 011.06.44l2.122 2.12a1.5 1.5 0 001.06.44H18A2.25 2.25 0 0120.25 9v.776"
      />
    </svg>
  );
}

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
    <div className="flex items-center -space-x-2">
      {allocations.slice(0, 4).map((a, i) => (
        <div
          key={a.contributor}
          className={`w-7 h-7 rounded-full ${colors[i % colors.length]} ring-2 ring-solana-card flex items-center justify-center text-[10px] font-bold text-white transition-transform duration-300 ease-spring hover:scale-110 hover:z-10`}
          title={`@${a.contributor}`}
        >
          {a.contributor.slice(0, 2).toUpperCase()}
        </div>
      ))}
      {allocations.length > 4 && (
        <div className="w-7 h-7 rounded-full bg-solana-dark ring-2 ring-solana-card flex items-center justify-center text-[10px] text-gray-400">
          +{allocations.length - 4}
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

  return (
    <Link to={`/campaign/${campaign.campaign_id}`} className="block group">
      <div className="card card-hover cursor-pointer h-full flex flex-col">
        {/* Header */}
        <div className="flex items-start justify-between mb-5">
          <div className="flex items-start gap-3 min-w-0">
            <div className="w-10 h-10 rounded-xl bg-solana-dark border border-solana-border flex items-center justify-center flex-shrink-0 group-hover:border-solana-purple/40 transition-all duration-300 ease-out-expo group-hover:shadow-md group-hover:shadow-solana-purple/10">
              <RepoIcon />
            </div>
            <div className="min-w-0">
              <h3 className="font-semibold text-base text-white truncate group-hover:text-solana-purple transition-colors duration-300">
                {campaign.repo}
              </h3>
              <p className="text-xs text-gray-500 font-mono mt-0.5 truncate">
                ID: {campaign.campaign_id.slice(0, 8)}...{campaign.campaign_id.slice(-4)}
              </p>
            </div>
          </div>
          <span className={`badge ${badgeClass} flex-shrink-0 ml-2`}>{stateConfig.label}</span>
        </div>

        {/* Details */}
        <div className="space-y-3 flex-1">
          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-500">Creator</span>
            <span className="text-gray-300 font-mono text-xs">
              {campaign.owner_github_username
                ? `@${campaign.owner_github_username}`
                : campaign.sponsor
                  ? `${campaign.sponsor.slice(0, 4)}...${campaign.sponsor.slice(-4)}`
                  : 'N/A'}
            </span>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-500">Pool</span>
            <span className="font-semibold text-solana-green flex items-center gap-1.5">
              <span className="w-2 h-2 rounded-full bg-solana-green inline-block animate-pulse" />
              {formatSOL(campaign.pool_amount)} SOL
            </span>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-500">Deadline</span>
            <span className="text-gray-300">{formatDate(campaign.deadline)}</span>
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between mt-5 pt-4 border-t border-solana-border">
          <ContributorAvatars allocations={campaign.allocations ?? []} />
          <span className="text-xs text-solana-purple font-medium group-hover:translate-x-1 transition-transform duration-300 ease-out-expo">
            DETAILS &rarr;
          </span>
        </div>
      </div>
    </Link>
  );
}
