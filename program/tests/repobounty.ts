import * as anchor from '@coral-xyz/anchor';
import { Program } from '@coral-xyz/anchor';
import { expect } from 'chai';

import { Repobounty } from '../target/types/repobounty';

const { Keypair, LAMPORTS_PER_SOL, PublicKey, SystemProgram } = anchor.web3;

function campaignPda(
  sponsor: PublicKey,
  campaignId: anchor.BN,
  programId: PublicKey
): [PublicKey, number] {
  return PublicKey.findProgramAddressSync(
    [
      Buffer.from('campaign'),
      sponsor.toBuffer(),
      campaignId.toArrayLike(Buffer, 'le', 8),
    ],
    programId
  );
}

function escrowPda(campaign: PublicKey, programId: PublicKey): [PublicKey, number] {
  return PublicKey.findProgramAddressSync(
    [Buffer.from('escrow'), campaign.toBuffer()],
    programId
  );
}

function claimRecordPda(
  campaign: PublicKey,
  githubUserId: anchor.BN,
  programId: PublicKey
): [PublicKey, number] {
  return PublicKey.findProgramAddressSync(
    [
      Buffer.from('claim'),
      campaign.toBuffer(),
      githubUserId.toArrayLike(Buffer, 'le', 8),
    ],
    programId
  );
}

async function airdrop(
  connection: anchor.web3.Connection,
  pubkey: PublicKey,
  sol: number
) {
  const signature = await connection.requestAirdrop(pubkey, sol * LAMPORTS_PER_SOL);
  await connection.confirmTransaction(signature, 'confirmed');
}

function expectErrorCode(error: unknown, expectedCode: string) {
  const anchorErrorCode = (error as any)?.error?.errorCode?.code;
  if (anchorErrorCode) {
    expect(anchorErrorCode).to.equal(expectedCode);
    return;
  }

  const message =
    error instanceof Error ? error.message : typeof error === "string" ? error : JSON.stringify(error);
  expect(message).to.include(expectedCode);
}

describe('repobounty', () => {
  const provider = anchor.AnchorProvider.env();
  anchor.setProvider(provider);

  const program = anchor.workspace.Repobounty as Program<Repobounty>;
  const connection = provider.connection;
  const programId = program.programId;

  const admin = Keypair.generate();
  const serviceWallet = Keypair.generate();
  const sponsor = Keypair.generate();
  const outsider = Keypair.generate();
  const contributor = Keypair.generate();

  const [configPda] = PublicKey.findProgramAddressSync([Buffer.from('config')], programId);

  const primaryCampaignId = new anchor.BN(1);
  const primaryGithubRepoId = new anchor.BN(123456);
  const rewardAmount = new anchor.BN(1_000_000_000);
  const serviceFeeAmount = 50_000_000;
  const deadlineOffsetSeconds = 10 * 60;

  before(async () => {
    await Promise.all(
      [admin, serviceWallet, sponsor, outsider, contributor].map((keypair) =>
        airdrop(connection, keypair.publicKey, 10)
      )
    );
  });

  describe('initialize_config', () => {
    it('creates config successfully', async () => {
      await program.methods
        .initializeConfig(
          serviceWallet.publicKey,
          serviceWallet.publicKey,
          serviceWallet.publicKey
        )
        .accounts({
          adminWallet: admin.publicKey,
          config: configPda,
          systemProgram: SystemProgram.programId,
        })
        .signers([admin])
        .rpc();

      const config = await program.account.config.fetch(configPda);
      expect(config.adminWallet.toBase58()).to.equal(admin.publicKey.toBase58());
      expect(config.finalizeAuthority.toBase58()).to.equal(
        serviceWallet.publicKey.toBase58()
      );
      expect(config.claimAuthority.toBase58()).to.equal(serviceWallet.publicKey.toBase58());
      expect(config.treasuryWallet.toBase58()).to.equal(
        serviceWallet.publicKey.toBase58()
      );
      expect(config.paused).to.equal(false);
    });

    it('rejects double initialization', async () => {
      try {
        await program.methods
          .initializeConfig(
            serviceWallet.publicKey,
            serviceWallet.publicKey,
            serviceWallet.publicKey
          )
          .accounts({
            adminWallet: admin.publicKey,
            config: configPda,
            systemProgram: SystemProgram.programId,
          })
          .signers([admin])
          .rpc();
        expect.fail('expected initializeConfig to fail');
      } catch {
        // expected: config PDA already exists
      }
    });
  });

  describe('admin controls', () => {
    it('rotates finalize, claim, and treasury wallets', async () => {
      const newFinalize = Keypair.generate().publicKey;
      const newClaim = Keypair.generate().publicKey;
      const newTreasury = Keypair.generate().publicKey;

      await program.methods
        .updateConfig(newFinalize, newClaim, newTreasury)
        .accounts({
          adminWallet: admin.publicKey,
          config: configPda,
        })
        .signers([admin])
        .rpc();

      let config = await program.account.config.fetch(configPda);
      expect(config.finalizeAuthority.toBase58()).to.equal(newFinalize.toBase58());
      expect(config.claimAuthority.toBase58()).to.equal(newClaim.toBase58());
      expect(config.treasuryWallet.toBase58()).to.equal(newTreasury.toBase58());

      await program.methods
        .updateConfig(
          serviceWallet.publicKey,
          serviceWallet.publicKey,
          serviceWallet.publicKey
        )
        .accounts({
          adminWallet: admin.publicKey,
          config: configPda,
        })
        .signers([admin])
        .rpc();

      config = await program.account.config.fetch(configPda);
      expect(config.finalizeAuthority.toBase58()).to.equal(
        serviceWallet.publicKey.toBase58()
      );
      expect(config.claimAuthority.toBase58()).to.equal(serviceWallet.publicKey.toBase58());
      expect(config.treasuryWallet.toBase58()).to.equal(
        serviceWallet.publicKey.toBase58()
      );
    });

    it('pauses and unpauses the program', async () => {
      await program.methods
        .setPaused(true)
        .accounts({
          adminWallet: admin.publicKey,
          config: configPda,
        })
        .signers([admin])
        .rpc();

      expect((await program.account.config.fetch(configPda)).paused).to.equal(true);

      await program.methods
        .setPaused(false)
        .accounts({
          adminWallet: admin.publicKey,
          config: configPda,
        })
        .signers([admin])
        .rpc();

      expect((await program.account.config.fetch(configPda)).paused).to.equal(false);
    });

    it('rejects unauthorized config updates', async () => {
      try {
        await program.methods
          .updateConfig(
            outsider.publicKey,
            outsider.publicKey,
            outsider.publicKey
          )
          .accounts({
            adminWallet: outsider.publicKey,
            config: configPda,
          })
          .signers([outsider])
          .rpc();
        expect.fail('expected updateConfig to fail');
      } catch (error) {
        expectErrorCode(error, 'Unauthorized');
      }
    });
  });

  describe('create_campaign_with_deposit', () => {
    it('creates campaign, funds escrow, and pays treasury fee', async () => {
      const deadline = new anchor.BN(Math.floor(Date.now() / 1000) + deadlineOffsetSeconds);
      const [campaign] = campaignPda(sponsor.publicKey, primaryCampaignId, programId);
      const [escrow] = escrowPda(campaign, programId);

      const treasuryBefore = await connection.getBalance(serviceWallet.publicKey);

      await program.methods
        .createCampaignWithDeposit(
          primaryCampaignId,
          primaryGithubRepoId,
          deadline,
          rewardAmount
        )
        .accounts({
          sponsor: sponsor.publicKey,
          config: configPda,
          campaign,
          escrow,
          treasuryWallet: serviceWallet.publicKey,
          systemProgram: SystemProgram.programId,
        })
        .signers([sponsor])
        .rpc();

      const createdCampaign = await program.account.campaign.fetch(campaign);
      expect(createdCampaign.campaignId.toNumber()).to.equal(primaryCampaignId.toNumber());
      expect(createdCampaign.sponsor.toBase58()).to.equal(sponsor.publicKey.toBase58());
      expect(createdCampaign.githubRepoId.toNumber()).to.equal(primaryGithubRepoId.toNumber());
      expect(createdCampaign.totalRewardAmount.toNumber()).to.equal(rewardAmount.toNumber());
      expect(createdCampaign.allocatedAmount.toNumber()).to.equal(0);
      expect(createdCampaign.claimedAmount.toNumber()).to.equal(0);
      expect(createdCampaign.allocationsCount).to.equal(0);
      expect(createdCampaign.claimedCount).to.equal(0);
      expect(createdCampaign.status).to.equal(0);

      const escrowBalance = await connection.getBalance(escrow);
      expect(escrowBalance).to.equal(rewardAmount.toNumber());

      const treasuryAfter = await connection.getBalance(serviceWallet.publicKey);
      expect(treasuryAfter - treasuryBefore).to.equal(serviceFeeAmount);
    });

    it('rejects deadline below minimum', async () => {
      const campaignId = new anchor.BN(2);
      const deadline = new anchor.BN(Math.floor(Date.now() / 1000) + 60);
      const [campaign] = campaignPda(sponsor.publicKey, campaignId, programId);
      const [escrow] = escrowPda(campaign, programId);

      try {
        await program.methods
          .createCampaignWithDeposit(
            campaignId,
            new anchor.BN(222),
            deadline,
            rewardAmount
          )
          .accounts({
            sponsor: sponsor.publicKey,
            config: configPda,
            campaign,
            escrow,
            treasuryWallet: serviceWallet.publicKey,
            systemProgram: SystemProgram.programId,
          })
          .signers([sponsor])
          .rpc();
        expect.fail('expected createCampaignWithDeposit to fail');
      } catch (error) {
        expectErrorCode(error, 'InvalidDeadline');
      }
    });

    it('rejects reward amount below minimum', async () => {
      const campaignId = new anchor.BN(3);
      const deadline = new anchor.BN(Math.floor(Date.now() / 1000) + deadlineOffsetSeconds);
      const [campaign] = campaignPda(sponsor.publicKey, campaignId, programId);
      const [escrow] = escrowPda(campaign, programId);

      try {
        await program.methods
          .createCampaignWithDeposit(
            campaignId,
            new anchor.BN(333),
            deadline,
            new anchor.BN(1)
          )
          .accounts({
            sponsor: sponsor.publicKey,
            config: configPda,
            campaign,
            escrow,
            treasuryWallet: serviceWallet.publicKey,
            systemProgram: SystemProgram.programId,
          })
          .signers([sponsor])
          .rpc();
        expect.fail('expected createCampaignWithDeposit to fail');
      } catch (error) {
        expectErrorCode(error, 'InvalidCampaignAmount');
      }
    });

    it('rejects an unexpected treasury wallet', async () => {
      const campaignId = new anchor.BN(4);
      const deadline = new anchor.BN(Math.floor(Date.now() / 1000) + deadlineOffsetSeconds);
      const [campaign] = campaignPda(sponsor.publicKey, campaignId, programId);
      const [escrow] = escrowPda(campaign, programId);

      try {
        await program.methods
          .createCampaignWithDeposit(
            campaignId,
            new anchor.BN(444),
            deadline,
            rewardAmount
          )
          .accounts({
            sponsor: sponsor.publicKey,
            config: configPda,
            campaign,
            escrow,
            treasuryWallet: outsider.publicKey,
            systemProgram: SystemProgram.programId,
          })
          .signers([sponsor])
          .rpc();
        expect.fail('expected createCampaignWithDeposit to fail');
      } catch (error) {
        expectErrorCode(error, 'Unauthorized');
      }
    });

    it('rejects campaign creation while paused', async () => {
      await program.methods
        .setPaused(true)
        .accounts({
          adminWallet: admin.publicKey,
          config: configPda,
        })
        .signers([admin])
        .rpc();

      const campaignId = new anchor.BN(5);
      const deadline = new anchor.BN(Math.floor(Date.now() / 1000) + deadlineOffsetSeconds);
      const [campaign] = campaignPda(sponsor.publicKey, campaignId, programId);
      const [escrow] = escrowPda(campaign, programId);

      try {
        await program.methods
          .createCampaignWithDeposit(
            campaignId,
            new anchor.BN(555),
            deadline,
            rewardAmount
          )
          .accounts({
            sponsor: sponsor.publicKey,
            config: configPda,
            campaign,
            escrow,
            treasuryWallet: serviceWallet.publicKey,
            systemProgram: SystemProgram.programId,
          })
          .signers([sponsor])
          .rpc();
        expect.fail('expected createCampaignWithDeposit to fail');
      } catch (error) {
        expectErrorCode(error, 'ProgramPaused');
      } finally {
        await program.methods
          .setPaused(false)
          .accounts({
            adminWallet: admin.publicKey,
            config: configPda,
          })
          .signers([admin])
          .rpc();
      }
    });
  });

  describe('finalize_campaign_batch', () => {
    it('rejects finalization before the deadline', async () => {
      const [campaign] = campaignPda(sponsor.publicKey, primaryCampaignId, programId);
      const githubUserId = new anchor.BN(9001);
      const [claimRecord] = claimRecordPda(campaign, githubUserId, programId);

      try {
        await program.methods
          .finalizeCampaignBatch(
            [{ githubUserId, amount: rewardAmount }],
            false
          )
          .accounts({
            finalizeAuthority: serviceWallet.publicKey,
            config: configPda,
            campaign,
            systemProgram: SystemProgram.programId,
          })
          .remainingAccounts([{ pubkey: claimRecord, isSigner: false, isWritable: true }])
          .signers([serviceWallet])
          .rpc();
        expect.fail('expected finalizeCampaignBatch to fail');
      } catch (error) {
        expectErrorCode(error, 'DeadlineNotReached');
      }
    });

    it('rejects the wrong finalize authority', async () => {
      const [campaign] = campaignPda(sponsor.publicKey, primaryCampaignId, programId);

      try {
        await program.methods
          .finalizeCampaignBatch(
            [{ githubUserId: new anchor.BN(9002), amount: rewardAmount }],
            false
          )
          .accounts({
            finalizeAuthority: outsider.publicKey,
            config: configPda,
            campaign,
            systemProgram: SystemProgram.programId,
          })
          .signers([outsider])
          .rpc();
        expect.fail('expected finalizeCampaignBatch to fail');
      } catch (error) {
        expectErrorCode(error, 'Unauthorized');
      }
    });
  });

  describe('clock-dependent scenarios', () => {
    it.skip('finalize_campaign_batch happy path after deadline', () => {});
    it.skip('claim happy path after finalization', async () => {
      const [campaign] = campaignPda(sponsor.publicKey, primaryCampaignId, programId);
      const githubUserId = new anchor.BN(9001);
      const [claimRecord] = claimRecordPda(campaign, githubUserId, programId);
      const [escrow] = escrowPda(campaign, programId);

      await program.methods
        .claim(githubUserId, 0)
        .accounts({
          user: contributor.publicKey,
          claimAuthority: serviceWallet.publicKey,
          config: configPda,
          campaign,
          claimRecord,
          escrow,
          recipientWallet: contributor.publicKey,
          systemProgram: SystemProgram.programId,
        })
        .signers([contributor, serviceWallet]);
    });
    it.skip('refund_unclaimed after claim window expires', async () => {
      const [campaign] = campaignPda(sponsor.publicKey, primaryCampaignId, programId);
      const [escrow] = escrowPda(campaign, programId);

      await program.methods
        .refundUnclaimed()
        .accounts({
          sponsor: sponsor.publicKey,
          config: configPda,
          campaign,
          escrow,
          systemProgram: SystemProgram.programId,
        })
        .signers([sponsor]);
    });
    it.skip('close_unfinalizable_campaign after deadline', () => {});
  });
});
