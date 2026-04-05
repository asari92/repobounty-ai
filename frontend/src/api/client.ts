import type {
  Campaign,
  CreateCampaignRequest,
  CreateCampaignResponse,
  WalletChallengeRequest,
  WalletChallengeResponse,
  FinalizePreviewResponse,
  FinalizeResponse,
  User,
  GitHubAuthRequest,
  GitHubAuthResponse,
  LinkWalletRequest,
  ClaimChallengeRequest,
  ClaimItem,
  BuildClaimTxResponse,
  HealthResponse,
} from '../types';

const API_BASE = '/api';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = localStorage.getItem('token');
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  if (options?.headers) {
    Object.entries(options.headers).forEach(([key, value]) => {
      headers[key] = String(value);
    });
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (!res.ok) {
    if (res.status === 401) {
      localStorage.removeItem('token');
      window.dispatchEvent(new Event('auth-expired'));
    }
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || `Request failed: ${res.status}`);
  }
  return res.json();
}

export const api = {
  createCampaignChallenge(data: WalletChallengeRequest): Promise<WalletChallengeResponse> {
    return request('/campaigns/create-challenge', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  createCampaign(data: CreateCampaignRequest): Promise<CreateCampaignResponse> {
    return request('/campaigns/', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  createCampaignConfirm(
    campaignId: string,
    data: {
      repo: string;
      pool_amount: number;
      deadline: string;
      sponsor_wallet: string;
      tx_signature: string;
    }
  ): Promise<CreateCampaignResponse> {
    return request(`/campaigns/${campaignId}/create-confirm`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  getCampaign(id: string): Promise<Campaign> {
    return request(`/campaigns/${id}`);
  },

  listCampaigns(): Promise<Campaign[]> {
    return request('/campaigns/');
  },

  finalizePreview(id: string): Promise<FinalizePreviewResponse> {
    return request(`/campaigns/${id}/finalize-preview`, { method: 'POST' });
  },

  finalize(id: string): Promise<FinalizeResponse> {
    return request(`/campaigns/${id}/finalize`, { method: 'POST' });
  },

  claimChallenge(
    campaignId: string,
    data: ClaimChallengeRequest
  ): Promise<WalletChallengeResponse> {
    return request(`/campaigns/${campaignId}/claim-challenge`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  claimAllocation(
    campaignId: string,
    contributorGithub: string,
    walletAddress: string,
    challengeId: string,
    signature: string
  ): Promise<BuildClaimTxResponse> {
    return request(`/campaigns/${campaignId}/claim`, {
      method: 'POST',
      body: JSON.stringify({
        contributor_github: contributorGithub,
        wallet_address: walletAddress,
        challenge_id: challengeId,
        signature,
      }),
    });
  },

  claimConfirm(
    campaignId: string,
    contributorGithub: string,
    walletAddress: string,
    txSignature: string
  ): Promise<FinalizeResponse> {
    return request(`/campaigns/${campaignId}/claim-confirm`, {
      method: 'POST',
      body: JSON.stringify({
        contributor_github: contributorGithub,
        wallet_address: walletAddress,
        tx_signature: txSignature,
      }),
    });
  },

  getGithubAuthUrl(): Promise<{ auth_url: string }> {
    return request('/auth/github/url');
  },

  githubCallback(data: GitHubAuthRequest): Promise<GitHubAuthResponse> {
    return request('/auth/github/callback', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  getMe(): Promise<User> {
    return request('/auth/me');
  },

  linkWallet(data: LinkWalletRequest): Promise<User> {
    return request('/auth/wallet/link', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  getClaims(): Promise<ClaimItem[]> {
    return request('/auth/claims');
  },
  getHealth(): Promise<HealthResponse> {
    return request('/health');
  },
};
