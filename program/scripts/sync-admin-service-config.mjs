import fs from 'fs';

import * as anchor from '@coral-xyz/anchor';

const { PublicKey, SystemProgram, Keypair, Connection } = anchor.web3;

function parseKeypair(value) {
  const trimmed = `${value ?? ''}`.trim();
  if (!trimmed) {
    throw new Error('missing keypair value');
  }

  try {
    const raw = anchor.utils.bytes.bs58.decode(trimmed);
    if (raw.length === 64) {
      return Keypair.fromSecretKey(Uint8Array.from(raw));
    }
  } catch {
    // Fallback to JSON parsing below.
  }

  const parsed = JSON.parse(trimmed);
  if (!Array.isArray(parsed) || parsed.length !== 64) {
    throw new Error('expected 64-byte keypair');
  }
  return Keypair.fromSecretKey(Uint8Array.from(parsed));
}

function readKeypairFile(path) {
  const parsed = JSON.parse(fs.readFileSync(path, 'utf8'));
  return Keypair.fromSecretKey(Uint8Array.from(parsed));
}

async function fetchConfigOrNull(program, configPda) {
  try {
    return await program.account.config.fetch(configPda);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if (
      message.includes('Account does not exist') ||
      message.includes('could not find account') ||
      message.includes('NotFound')
    ) {
      return null;
    }
    throw error;
  }
}

async function main() {
  const rpcUrl = process.env.ANCHOR_PROVIDER_URL || process.env.SOLANA_RPC_URL;
  const adminWalletPath = process.env.ANCHOR_WALLET || '/wallet/id.json';
  const servicePrivateKey = process.env.SERVICE_PRIVATE_KEY || '';
  const idlPath = '/app/target/idl/repobounty.json';

  if (!rpcUrl) {
    throw new Error('missing ANCHOR_PROVIDER_URL or SOLANA_RPC_URL');
  }
  if (!servicePrivateKey.trim()) {
    throw new Error('missing SERVICE_PRIVATE_KEY');
  }

  const adminKeypair = readKeypairFile(adminWalletPath);
  const serviceKeypair = parseKeypair(servicePrivateKey);
  const idl = JSON.parse(fs.readFileSync(idlPath, 'utf8'));

  const connection = new Connection(rpcUrl, 'confirmed');
  const provider = new anchor.AnchorProvider(connection, new anchor.Wallet(adminKeypair), {
    commitment: 'confirmed',
  });
  anchor.setProvider(provider);

  const program = new anchor.Program(idl, provider);
  const [configPda] = PublicKey.findProgramAddressSync([Buffer.from('config')], program.programId);

  console.log(`Program ID: ${program.programId.toBase58()}`);
  console.log(`Admin wallet: ${adminKeypair.publicKey.toBase58()}`);
  console.log(`Service wallet: ${serviceKeypair.publicKey.toBase58()}`);

  const existingConfig = await fetchConfigOrNull(program, configPda);
  if (!existingConfig) {
    await program.methods
      .initializeConfig(
        serviceKeypair.publicKey,
        serviceKeypair.publicKey,
        serviceKeypair.publicKey
      )
      .accounts({
        config: configPda,
        adminWallet: adminKeypair.publicKey,
        systemProgram: SystemProgram.programId,
      })
      .signers([adminKeypair])
      .rpc();
    console.log('Initialized config with admin wallet and service wallet authorities.');
    return;
  }

  if (existingConfig.adminWallet.toBase58() !== adminKeypair.publicKey.toBase58()) {
    throw new Error(
      `config admin ${existingConfig.adminWallet.toBase58()} does not match current admin wallet ${adminKeypair.publicKey.toBase58()}`
    );
  }

  const finalizeMatches =
    existingConfig.finalizeAuthority.toBase58() === serviceKeypair.publicKey.toBase58();
  const claimMatches =
    existingConfig.claimAuthority.toBase58() === serviceKeypair.publicKey.toBase58();
  const treasuryMatches =
    existingConfig.treasuryWallet.toBase58() === serviceKeypair.publicKey.toBase58();

  if (finalizeMatches && claimMatches && treasuryMatches) {
    console.log('Config already matches admin/service wallet roles.');
    return;
  }

  await program.methods
    .updateConfig(
      serviceKeypair.publicKey,
      serviceKeypair.publicKey,
      serviceKeypair.publicKey
    )
    .accounts({
      config: configPda,
      adminWallet: adminKeypair.publicKey,
    })
    .signers([adminKeypair])
    .rpc();

  console.log('Updated config finalize_authority, claim_authority, and treasury_wallet to service wallet.');
}

await main();
