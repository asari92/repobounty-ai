import * as anchor from "@coral-xyz/anchor";
import { Program } from "@coral-xyz/anchor";
import { expect } from "chai";

import type { Repobounty } from "../target/types/repobounty";

const DAY_IN_SECONDS = 24 * 60 * 60;
const BN = (
  anchor as typeof anchor & {
    default: { BN: typeof anchor.BN };
  }
).default.BN;
const POOL_AMOUNT = new BN(1_000_000_000);

function deadlineAtLeast24HoursOut(): anchor.BN {
  return new BN(Math.floor(Date.now() / 1000) + DAY_IN_SECONDS + 60);
}

function deadlineTooSoon(): anchor.BN {
  return new BN(Math.floor(Date.now() / 1000) + DAY_IN_SECONDS - 60);
}

function deriveCampaignAddresses(
  programId: anchor.web3.PublicKey,
  campaignId: string,
): { campaignPda: anchor.web3.PublicKey; vaultPda: anchor.web3.PublicKey } {
  const [campaignPda] = anchor.web3.PublicKey.findProgramAddressSync(
    [Buffer.from("campaign"), Buffer.from(campaignId)],
    programId,
  );
  const [vaultPda] = anchor.web3.PublicKey.findProgramAddressSync(
    [Buffer.from("vault"), campaignPda.toBuffer()],
    programId,
  );

  return { campaignPda, vaultPda };
}

async function expectAnchorError(
  action: Promise<unknown>,
  expectedCode: string,
) {
  try {
    await action;
    expect.fail(`expected ${expectedCode}`);
  } catch (err) {
    expect((err as any)?.error?.errorCode?.code).to.equal(expectedCode);
  }
}

describe("repobounty", () => {
  const provider = anchor.AnchorProvider.env();
  anchor.setProvider(provider);

  const program = anchor.workspace.Repobounty as Program<Repobounty>;
  const authority = provider.wallet.publicKey;
  const sponsor = provider.wallet.publicKey;
  const repo = "anthropics/claude-code";

  async function createCampaign(campaignId: string, deadline: anchor.BN) {
    const { campaignPda, vaultPda } = deriveCampaignAddresses(
      program.programId,
      campaignId,
    );

    const signature = await program.methods
      .createCampaign(campaignId, repo, POOL_AMOUNT, deadline, sponsor)
      .accounts({
        campaign: campaignPda,
        authority,
        vault: vaultPda,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .rpc();

    return { campaignPda, vaultPda, signature };
  }

  async function fundCampaign(
    campaignPda: anchor.web3.PublicKey,
    vaultPda: anchor.web3.PublicKey,
  ) {
    const transferIx = anchor.web3.SystemProgram.transfer({
      fromPubkey: sponsor,
      toPubkey: vaultPda,
      lamports: POOL_AMOUNT.toNumber(),
    });

    return program.methods
      .fundCampaign()
      .accounts({
        campaign: campaignPda,
        vault: vaultPda,
        sponsor,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .preInstructions([transferIx])
      .rpc();
  }

  it("creates campaign with sponsor and derived vault PDA", async () => {
    const campaignId = "test-campaign-create";
    const { campaignPda, signature } = await createCampaign(
      campaignId,
      deadlineAtLeast24HoursOut(),
    );

    expect(signature).to.be.a("string").and.not.empty;

    const campaign = await program.account.campaign.fetch(campaignPda);
    expect(campaign.repo).to.equal(repo);
    expect(campaign.poolAmount.toNumber()).to.equal(POOL_AMOUNT.toNumber());
    expect(campaign.sponsor.toBase58()).to.equal(sponsor.toBase58());
    expect(campaign.authority.toBase58()).to.equal(authority.toBase58());
    expect(campaign.state).to.deep.equal({ created: {} });
    expect(campaign.allocations).to.have.length(0);
  });

  it("rejects creating a campaign with a deadline shorter than 24 hours", async () => {
    await expectAnchorError(
      createCampaign("test-campaign-too-soon", deadlineTooSoon()),
      "DeadlineTooSoon",
    );
  });

  it("funds a created campaign with a transfer plus fund_campaign instruction", async () => {
    const { campaignPda, vaultPda } = await createCampaign(
      "test-campaign-funded",
      deadlineAtLeast24HoursOut(),
    );

    const signature = await fundCampaign(campaignPda, vaultPda);

    expect(signature).to.be.a("string").and.not.empty;

    const campaign = await program.account.campaign.fetch(campaignPda);
    expect(campaign.state).to.deep.equal({ funded: {} });

    const vaultBalance = await provider.connection.getBalance(vaultPda);
    expect(vaultBalance).to.be.at.least(POOL_AMOUNT.toNumber());
  });

  it("rejects funding a campaign twice", async () => {
    const { campaignPda, vaultPda } = await createCampaign(
      "test-campaign-double-fund",
      deadlineAtLeast24HoursOut(),
    );

    await fundCampaign(campaignPda, vaultPda);

    await expectAnchorError(
      program.methods
        .fundCampaign()
        .accounts({
          campaign: campaignPda,
          vault: vaultPda,
          sponsor,
          systemProgram: anchor.web3.SystemProgram.programId,
        })
        .rpc(),
      "InvalidCampaignState",
    );
  });

  it("rejects finalizing before the deadline even after funding", async () => {
    const { campaignPda, vaultPda } = await createCampaign(
      "test-campaign-before-deadline",
      deadlineAtLeast24HoursOut(),
    );

    await fundCampaign(campaignPda, vaultPda);

    await expectAnchorError(
      program.methods
        .finalizeCampaign([{ contributor: "alice", percentage: 10000 }])
        .accounts({
          campaign: campaignPda,
          authority,
        })
        .rpc(),
      "DeadlineNotReached",
    );
  });

  it("rejects finalizing an unfunded campaign", async () => {
    const { campaignPda } = await createCampaign(
      "test-campaign-unfunded",
      deadlineAtLeast24HoursOut(),
    );

    await expectAnchorError(
      program.methods
        .finalizeCampaign([{ contributor: "alice", percentage: 10000 }])
        .accounts({
          campaign: campaignPda,
          authority,
        })
        .rpc(),
      "InvalidCampaignState",
    );
  });

  it("builds claim instructions with the backend authority signer account", async () => {
    const { campaignPda, vaultPda } = deriveCampaignAddresses(
      program.programId,
      "test-campaign-claim-ix",
    );
    const contributor = anchor.web3.Keypair.generate().publicKey;

    const instruction = await program.methods
      .claim("alice")
      .accounts({
        campaign: campaignPda,
        vault: vaultPda,
        authority,
        contributor,
        systemProgram: anchor.web3.SystemProgram.programId,
      })
      .instruction();

    expect(instruction.keys).to.have.length(5);
    expect(instruction.keys[0].pubkey.toBase58()).to.equal(
      campaignPda.toBase58(),
    );
    expect(instruction.keys[1].pubkey.toBase58()).to.equal(vaultPda.toBase58());
    expect(instruction.keys[2].pubkey.toBase58()).to.equal(
      authority.toBase58(),
    );
    expect(instruction.keys[2].isSigner).to.equal(true);
    expect(instruction.keys[3].pubkey.toBase58()).to.equal(
      contributor.toBase58(),
    );
  });
});
