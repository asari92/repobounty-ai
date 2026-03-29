import * as anchor from "@coral-xyz/anchor";
import { Program } from "@coral-xyz/anchor";
import { Repobounty } from "../target/types/repobounty";
import { expect } from "chai";

describe("repobounty", () => {
  const provider = anchor.AnchorProvider.env();
  anchor.setProvider(provider);

  const program = anchor.workspace.Repobounty as Program<Repobounty>;
  const authority = provider.wallet.publicKey;
  const sponsor = provider.wallet.publicKey;

  const campaignId = "test-campaign-001";
  const repo = "anthropics/claude-code";
  const poolAmount = new anchor.BN(1_000_000_000); // 1 SOL in lamports
  const deadline = new anchor.BN(Math.floor(Date.now() / 1000) + 86400); // +24h

  let campaignPda: anchor.web3.PublicKey;
  let campaignBump: number;
  let vaultPda: anchor.web3.PublicKey;
  let vaultBump: number;

  before(async () => {
    [campaignPda, campaignBump] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("campaign"), Buffer.from(campaignId)],
      program.programId
    );
    [vaultPda, vaultBump] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("vault"), campaignPda.toBuffer()],
      program.programId
    );
  });

  it("creates campaign with sponsor and vault", async () => {
    const tx = await program.methods
      .createCampaign(campaignId, repo, poolAmount, deadline, sponsor)
      .accounts({
        campaign: campaignPda,
        authority: authority,
        vault: vaultPda,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .rpc();

    console.log("  create_campaign tx:", tx);

    const campaign = await program.account.campaign.fetch(campaignPda);
    expect(campaign.repo).to.equal(repo);
    expect(campaign.poolAmount.toNumber()).to.equal(poolAmount.toNumber());
    expect(campaign.sponsor.toBase58()).to.equal(sponsor.toBase58());
    expect(campaign.authority.toBase58()).to.equal(authority.toBase58());
    expect(campaign.state).to.deep.equal({ created: {} });
    expect(campaign.allocations).to.have.length(0);
  });

  it("funds campaign with SOL transfer", async () => {
    const transferIx = anchor.web3.SystemProgram.transfer({
      fromPubkey: sponsor,
      toPubkey: vaultPda,
      lamports: poolAmount.toNumber(),
    });

    const fundTx = await program.methods
      .fundCampaign()
      .accounts({
        campaign: campaignPda,
        vault: vaultPda,
        sponsor: sponsor,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .preInstructions([transferIx])
      .rpc();

    console.log("  fund_campaign tx:", fundTx);

    const campaign = await program.account.campaign.fetch(campaignPda);
    expect(campaign.state).to.deep.equal({ funded: {} });

    const vaultBalance = await provider.connection.getBalance(vaultPda);
    expect(vaultBalance).to.be.at.least(poolAmount.toNumber());
  });

  it("rejects funding already funded campaign", async () => {
    try {
      await program.methods
        .fundCampaign()
        .accounts({
          campaign: campaignPda,
          vault: vaultPda,
          sponsor: sponsor,
          systemProgram: anchor.web3.SystemProgram.programId,
        })
        .rpc();
      expect.fail("should have thrown");
    } catch (err: any) {
      expect(err.error.errorCode.code).to.equal("AlreadyFunded");
    }
  });

  it("rejects finalizing unfunded campaign", async () => {
    const id2 = "test-campaign-002";
    const [pda2] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("campaign"), Buffer.from(id2)],
      program.programId
    );
    const [vault2] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("vault"), pda2.toBuffer()],
      program.programId
    );

    await program.methods
      .createCampaign(id2, repo, poolAmount, deadline, sponsor)
      .accounts({
        campaign: pda2,
        authority: authority,
        vault: vault2,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .rpc();

    try {
      await program.methods
        .finalizeCampaign([{ contributor: "alice", percentage: 10000 }])
        .accounts({
          campaign: pda2,
          authority: authority,
        })
        .rpc();
      expect.fail("should have thrown");
    } catch (err: any) {
      expect(err.error.errorCode.code).to.equal("AlreadyFinalized");
    }
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
        authority: authority,
      })
      .rpc();

    console.log("  finalize_campaign tx:", tx);

    const campaign = await program.account.campaign.fetch(campaignPda);
    expect(campaign.state).to.deep.equal({ finalized: {} });
    expect(campaign.allocations).to.have.length(3);

    expect(campaign.allocations[0].contributor).to.equal("alice");
    expect(campaign.allocations[0].percentage).to.equal(5000);
    expect(campaign.allocations[0].amount.toNumber()).to.equal(500_000_000);
    expect(campaign.allocations[0].claimed).to.equal(false);
    expect(campaign.allocations[0].claimant).to.be.null;

    expect(campaign.allocations[1].contributor).to.equal("bob");
    expect(campaign.allocations[1].amount.toNumber()).to.equal(300_000_000);

    expect(campaign.allocations[2].contributor).to.equal("charlie");
    expect(campaign.allocations[2].amount.toNumber()).to.equal(200_000_000);
  });

  it("claims allocation successfully", async () => {
    const contributor = provider.wallet.publicKey;
    const beforeBalance = await provider.connection.getBalance(contributor);

    const claimTx = await program.methods
      .claim("alice")
      .accounts({
        campaign: campaignPda,
        vault: vaultPda,
        contributor: contributor,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .rpc();

    console.log("  claim tx:", claimTx);

    const campaign = await program.account.campaign.fetch(campaignPda);
    const allocation = campaign.allocations.find((a: any) => a.contributor === "alice");
    expect(allocation.claimed).to.equal(true);
    expect(allocation.claimant.toBase58()).to.equal(contributor.toBase58());

    const afterBalance = await provider.connection.getBalance(contributor);
    expect(afterBalance - beforeBalance).to.equal(500_000_000);
  });

  it("rejects double claim", async () => {
    try {
      await program.methods
        .claim("alice")
        .accounts({
          campaign: campaignPda,
          vault: vaultPda,
          contributor: provider.wallet.publicKey,
          systemProgram: anchor.web3.SystemProgram.programId,
        })
        .rpc();
      expect.fail("should have thrown");
    } catch (err: any) {
      expect(err.error.errorCode.code).to.equal("AlreadyClaimed");
    }
  });

  it("transitions to Completed when all claimed", async () => {
    for (const contrib of ["bob", "charlie"]) {
      await program.methods
        .claim(contrib)
        .accounts({
          campaign: campaignPda,
          vault: vaultPda,
          contributor: provider.wallet.publicKey,
          systemProgram: anchor.web3.SystemProgram.programId,
        })
        .rpc();
    }

    const campaign = await program.account.campaign.fetch(campaignPda);
    expect(campaign.state).to.deep.equal({ completed: {} });
    expect(campaign.totalClaimed.toNumber()).to.equal(1_000_000_000);
  });

  it("rejects allocations not summing to 100%", async () => {
    const id3 = "test-campaign-003";
    const [pda3] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("campaign"), Buffer.from(id3)],
      program.programId
    );
    const [vault3] = anchor.web3.PublicKey.findProgramAddressSync(
      [Buffer.from("vault"), pda3.toBuffer()],
      program.programId
    );

    await program.methods
      .createCampaign(id3, repo, poolAmount, deadline, sponsor)
      .accounts({
        campaign: pda3,
        authority: authority,
        vault: vault3,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .rpc();

    const transferIx = anchor.web3.SystemProgram.transfer({
      fromPubkey: sponsor,
      toPubkey: vault3,
      lamports: poolAmount.toNumber(),
    });

    await program.methods
      .fundCampaign()
      .accounts({
        campaign: pda3,
        vault: vault3,
        sponsor: sponsor,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .preInstructions([transferIx])
      .rpc();

    try {
      await program.methods
        .finalizeCampaign([
          { contributor: "alice", percentage: 5000 },
          { contributor: "bob", percentage: 3000 },
        ])
        .accounts({
          campaign: pda3,
          authority: authority,
        })
        .rpc();
      expect.fail("should have thrown");
    } catch (err: any) {
      expect(err.error.errorCode.code).to.equal("InvalidAllocationTotal");
    }
  });
});
