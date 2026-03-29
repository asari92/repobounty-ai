import type {
  Campaign,
  CreateCampaignRequest,
  CreateCampaignResponse,
  FinalizePreviewResponse,
  FinalizeResponse,
} from "../types";

const API_BASE = "/api";

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || `Request failed: ${res.status}`);
  }
  return res.json();
}

export const api = {
  createCampaign(data: CreateCampaignRequest): Promise<CreateCampaignResponse> {
    return request("/campaigns/", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  getCampaign(id: string): Promise<Campaign> {
    return request(`/campaigns/${id}`);
  },

  listCampaigns(): Promise<Campaign[]> {
    return request("/campaigns/");
  },

  finalizePreview(id: string): Promise<FinalizePreviewResponse> {
    return request(`/campaigns/${id}/finalize-preview`, { method: "POST" });
  },

  finalize(id: string): Promise<FinalizeResponse> {
    return request(`/campaigns/${id}/finalize`, { method: "POST" });
  },
};
