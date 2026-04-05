use anchor_lang::prelude::*;

#[event]
pub struct CampaignCreated {
    pub campaign_pubkey: Pubkey,
    pub campaign_id: u64,
    pub sponsor: Pubkey,
    pub github_repo_id: u64,
    pub deadline_at: i64,
    pub total_reward_amount: u64,
    pub service_fee: u64,
}

#[event]
pub struct FinalizeBatchAppended {
    pub campaign_pubkey: Pubkey,
    pub batch_count: u32,
    pub batch_total_amount: u64,
    pub has_more: bool,
}

#[event]
pub struct CampaignFinalized {
    pub campaign_pubkey: Pubkey,
    pub allocations_count: u32,
    pub allocated_amount: u64,
}

#[event]
pub struct ClaimProcessed {
    pub campaign_pubkey: Pubkey,
    pub github_user_id: u64,
    pub recipient_wallet: Pubkey,
    pub amount: u64,
}

#[event]
pub struct CampaignClosed {
    pub campaign_pubkey: Pubkey,
    pub reason: String,
}

#[event]
pub struct RefundProcessed {
    pub campaign_pubkey: Pubkey,
    pub sponsor: Pubkey,
    pub refunded_amount: u64,
}
