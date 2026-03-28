export interface Campaign {
  campaign_id: string;
  repo: string;
  pool_amount: number;
  deadline: string;
  state: "created" | "finalized";
  authority: string;
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
  wallet_address: string;
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
  warning?: string;
}

export interface ApiError {
  error: string;
  details?: string;
}
