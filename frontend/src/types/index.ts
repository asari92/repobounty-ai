export interface Campaign {
  campaign_id: string;
  campaign_pda?: string;
  vault_address?: string;
  repo: string;
  pool_amount: number;
  total_claimed: number;
  deadline: string;
  state: 'created' | 'funded' | 'finalized' | 'completed';
  authority: string;
  sponsor: string;
  owner_github_username?: string;
  allocations: Allocation[];
  created_at: string;
  finalized_at?: string;
  tx_signature?: string;
}

export interface Allocation {
  contributor: string;
  percentage: number;
  amount: number;
  reasoning?: string;
  claimed: boolean;
  claimant_wallet?: string;
}

export interface Contributor {
  username: string;
  commits: number;
  pull_requests: number;
  reviews: number;
  lines_added: number;
  lines_deleted: number;
}

export interface CreateCampaignRequest {
  repo: string;
  pool_amount: number;
  deadline: string;
  sponsor_wallet: string;
}

export interface CreateCampaignResponse {
  campaign_id: string;
  campaign_pda: string;
  vault_address: string;
  escrow_pda?: string;
  repo: string;
  pool_amount: number;
  deadline: string;
  state?: 'created' | 'funded' | 'finalized' | 'completed';
  status?: 'active' | 'finalized' | 'closed';
  tx_signature?: string;
  unsigned_tx?: string;
}

export interface FinalizePreviewResponse {
  campaign_id: string;
  repo: string;
  contributors: Contributor[];
  allocations: Allocation[];
  ai_model: string;
  allocation_mode: 'code_impact' | 'metrics';
  snapshot: SnapshotSummary;
}

export interface FinalizeResponse {
  campaign_id: string;
  state: 'finalized' | 'completed';
  allocations: Allocation[];
  tx_signature: string;
  solana_explorer_url: string;
  allocation_mode?: 'code_impact' | 'metrics';
  snapshot?: SnapshotSummary;
}

export interface ApiError {
  error: string;
  details?: string;
}

export interface User {
  github_username: string;
  github_id: number;
  avatar_url: string;
  wallet_address?: string;
  created_at: string;
}

export interface GitHubAuthRequest {
  code: string;
  state?: string;
}

export interface GitHubAuthResponse {
  token: string;
  user: User;
}

export interface LinkWalletRequest {
  wallet_address: string;
}

export interface WalletChallengeRequest {
  repo: string;
  pool_amount: number;
  deadline: string;
  sponsor_wallet: string;
}

export interface ClaimChallengeRequest {
  contributor_github: string;
  wallet_address: string;
}

export interface WalletChallengeResponse {
  challenge_id: string;
  action: 'create_campaign' | 'claim';
  wallet_address: string;
  message: string;
  expires_at: string;
}

export interface BuildClaimTxResponse {
  partial_tx: string;
}

export interface SnapshotSummary {
  version: number;
  allocation_mode: 'code_impact' | 'metrics';
  window_start: string;
  window_end: string;
  contributor_source: string;
  contributor_notes?: string;
  created_at: string;
  approved_by_github_username?: string;
  approved_at?: string;
}

export interface ClaimItem {
  campaign_id: string;
  repo: string;
  contributor: string;
  percentage: number;
  amount: number;
  amount_sol: string;
  claimed: boolean;
  claimant_wallet?: string;
  state: string;
}

export interface MyCampaign {
  campaign_id: string;
  campaign_pda?: string;
  repo: string;
  pool_amount: number;
  state: string;
  status?: string;
  sponsor: string;
  authority: string;
  owner_github_username?: string;
  allocations: Allocation[];
  created_at: string;
  deadline: string;
  can_refund: boolean;
}

export interface FundTransactionResponse {
  transaction: string;
  campaign_pda: string;
  vault_address: string;
}

export interface HealthResponse {
  status: string;
  solana: boolean;
  github: boolean;
  ai_model: string;
  store: boolean;
}

export interface UserSearchResult {
  login: string;
  avatar_url: string;
}

export interface RepoSearchResult {
  name: string;
  owner: string;
}
