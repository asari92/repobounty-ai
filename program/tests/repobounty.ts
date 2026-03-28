import * as anchor from "@coral-xyz/anchor";
import { Program } from "@coral-xyz/anchor";
import { Repobounty } from "../target/types/repobounty";
import { expect } from "chai";

describe("repobounty", () => {
  const provider = anchor.AnchorProvider.env();
  anchor.setProvider(provider);

  const program = anchor.workspace.Repobounty as Program<Repobounty>;
  const authority = provider.wallet;

  const campaignId = "test-campaign-001";
  const repo = "anthropics/claude-code";
  const poolAmount = new anchor.BN(1_000_000_000); // 1 SOL in lamports
  const deadline = new anchor.BN(Math.floor(Date.now() / 1000) + 86400); // +24h

  let campaignPda: anchor.web3.PublicKey;
  let campaignBump: number;

  before(async () => {
    [campaignPda, campaignBump] = anchor.web3.PublicKey.findProgramAddressSync(
      [
        Buffer.from("campaign"),
        authority.publicKey.toBuffer(),
        Buffer.from(campaignId),
      ],
      program.programId
    );
  });

  it("creates a campaign", async () => {
    const tx = await program.methods
      .createCampaign(campaignId, repo, poolAmount, deadline)
      .accounts({
        campaign: campaignPda,
        authority: authority.publicKey,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .rpc();

    console.log("  create_campaign tx:", tx);

    const campaign = await program.account.campaign.fetch(campaignPda);
    expect(campaign.repo).to.equal(repo);
    expect(campaign.poolAmount.toNumber()).to.equal(poolAmount.toNumber());
    expect(campaign.state).to.deep.equal({ created: {} });
    expect(campaign.allocations).to.have.length(0);
  });

  it("finalizes a campaign with allocations", async () => {
    const allocations = [
      { contributor: "alice", percentage: 5000 },
      { contributor: "bob", percentage: 3000 },
      { contributor: "charlie", percentage: 2000 },
    ];

    const tx = await program.methods
      .finalizeCampaign(allocations)
      .accounts({
        campaign: campaignPda,
        authority: authority.publicKey,
      })
      .rpc();

    console.log("  finalize_campaign tx:", tx);

    const campaign = await program.account.campaign.fetch(campaignPda);
    expect(campaign.state).to.deep.equal({ finalized: {} });
    expect(campaign.allocations).to.have.length(3);

    // alice: 50% of 1 SOL = 0.5 SOL
    expect(campaign.allocations[0].contributor).to.equal("alice");
    expect(campaign.allocations[0].percentage).to.equal(5000);
    expect(campaign.allocations[0].amount.toNumber()).to.equal(500_000_000);

    // bob: 30%
    expect(campaign.allocations[1].contributor).to.equal("bob");
    expect(campaign.allocations[1].amount.toNumber()).to.equal(300_000_000);

    // charlie: 20%
    expect(campaign.allocations[2].contributor).to.equal("charlie");
    expect(campaign.allocations[2].amount.toNumber()).to.equal(200_000_000);

    expect(campaign.finalizedAt).to.not.be.null;
  });

  it("rejects double finalization", async () => {
    try {
      await program.methods
        .finalizeCampaign([{ contributor: "eve", percentage: 10000 }])
        .accounts({
          campaign: campaignPda,
          authority: authority.publicKey,
        })
        .rpc();
      expect.fail("should have thrown");
    } catch (err: any) {
      expect(err.error.errorCode.code).to.equal("CampaignAlreadyFinalized");
    }
  });

  it("rejects allocations not summing to 100%", async () => {
    // Create a new campaign for this test
    const id2 = "test-campaign-002";
    const [pda2] = anchor.web3.PublicKey.findProgramAddressSync(
      [
        Buffer.from("campaign"),
        authority.publicKey.toBuffer(),
        Buffer.from(id2),
      ],
      program.programId
    );

    await program.methods
      .createCampaign(id2, repo, poolAmount, deadline)
      .accounts({
        campaign: pda2,
        authority: authority.publicKey,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .rpc();

    try {
      await program.methods
        .finalizeCampaign([
          { contributor: "alice", percentage: 5000 },
          { contributor: "bob", percentage: 3000 },
          // Only 80% — should fail
        ])
        .accounts({
          campaign: pda2,
          authority: authority.publicKey,
        })
        .rpc();
      expect.fail("should have thrown");
    } catch (err: any) {
      expect(err.error.errorCode.code).to.equal("InvalidAllocationTotal");
    }
  });
});
