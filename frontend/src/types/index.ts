export interface Campaign {
  campaign_id: string;
  campaign_pda?: string;
  vault_address?: string;
  repo: string;
  pool_amount: number;
  total_claimed: number;
  deadline: string;
  state: "created" | "funded" | "finalized" | "completed";
  authority: string;
  sponsor: string;
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
  repo: string;
  pool_amount: number;
  deadline: string;
  state: "created";
  tx_signature: string;
}

export interface FinalizePreviewResponse {
  campaign_id: string;
  repo: string;
  contributors: Contributor[];
  allocations: Allocation[];
  ai_model: string;
}

export interface FinalizeResponse {
  campaign_id: string;
  state: "finalized";
  allocations: Allocation[];
  tx_signature: string;
  solana_explorer_url: string;
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

export interface FundTransactionResponse {
  transaction: string;
  campaign_pda: string;
  vault_address: string;
}
