package solana

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	rpcjson "github.com/gagliardetto/solana-go/rpc/jsonrpc"
	"github.com/mr-tron/base58"
	"github.com/repobounty/repobounty-ai/internal/models"
)

type Client struct {
	rpcClient  *rpc.Client
	privateKey solana.PrivateKey
	programID  solana.PublicKey
}

var campaignAccountDiscriminator = anchorDiscriminator("account:Campaign")
var claimRecordAccountDiscriminator = anchorDiscriminator("account:ClaimRecord")
var ErrNotConfigured = errors.New("solana client not configured")
var ErrCampaignNotFound = errors.New("campaign not found")

const (
	campaignAccountSpace  = 171
	serviceFeeDenominator = 1000
	serviceFeeNumerator   = 5
	minServiceFeeLamports = 50_000_000
)

func NewClient(rpcURL, privateKeyBase58, programIDStr string) (*Client, error) {
	if strings.TrimSpace(privateKeyBase58) == "" || isPlaceholderProgramID(programIDStr) {
		log.Printf("solana: no credentials configured, using mock mode")
		return &Client{}, nil
	}

	privKey, err := parsePrivateKey(privateKeyBase58)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	programID, err := solana.PublicKeyFromBase58(programIDStr)
	if err != nil {
		return nil, fmt.Errorf("parse program ID: %w", err)
	}

	return &Client{
		rpcClient:  rpc.New(rpcURL),
		privateKey: privKey,
		programID:  programID,
	}, nil
}

func parsePrivateKey(value string) (solana.PrivateKey, error) {
	value = strings.TrimSpace(value)

	privKey, err := solana.PrivateKeyFromBase58(value)
	if err == nil {
		return privKey, nil
	}

	var raw []byte
	if jsonErr := json.Unmarshal([]byte(value), &raw); jsonErr != nil {
		return nil, err
	}
	if len(raw) != 64 {
		return nil, fmt.Errorf("expected 64-byte keypair, got %d bytes", len(raw))
	}

	return solana.PrivateKey(raw), nil
}

func isPlaceholderProgramID(programID string) bool {
	switch strings.TrimSpace(programID) {
	case "", "11111111111111111111111111111111", "11111111111111111111111111111112":
		return true
	default:
		return false
	}
}

func (c *Client) IsConfigured() bool {
	return c.rpcClient != nil
}

func (c *Client) AuthorityAddress() string {
	if !c.IsConfigured() {
		return ""
	}
	return c.privateKey.PublicKey().String()
}

func (c *Client) ListCampaigns(ctx context.Context) ([]*models.Campaign, error) {
	if !c.IsConfigured() {
		return nil, nil
	}

	accounts, err := c.rpcClient.GetProgramAccountsWithOpts(
		ctx,
		c.programID,
		&rpc.GetProgramAccountsOpts{Commitment: rpc.CommitmentConfirmed},
	)
	if err != nil {
		return nil, fmt.Errorf("get program accounts: %w", err)
	}

	campaigns := make([]*models.Campaign, 0, len(accounts))
	for _, acct := range accounts {
		if acct == nil || acct.Account == nil {
			continue
		}

		data := acct.Account.Data.GetBinary()
		campaign, err := decodeCampaignAccount(data, acct.Pubkey.String(), c.programID)
		if err != nil {
			log.Printf("solana: skip undecodable campaign account %s: %v", acct.Pubkey.String(), err)
			continue
		}
		campaigns = append(campaigns, campaign)
	}

	sort.Slice(campaigns, func(i, j int) bool {
		return campaigns[i].CreatedAt.After(campaigns[j].CreatedAt)
	})

	return campaigns, nil
}

func (c *Client) GetCampaign(ctx context.Context, campaignID string) (*models.Campaign, error) {
	campaigns, err := c.ListCampaigns(ctx)
	if err != nil {
		return nil, err
	}

	for _, campaign := range campaigns {
		if campaign.CampaignID == campaignID {
			return campaign, nil
		}
	}

	return nil, ErrCampaignNotFound
}

func parseCampaignID(campaignID string) (uint64, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(campaignID), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse numeric campaign id: %w", err)
	}
	if parsed == 0 {
		return 0, fmt.Errorf("campaign id must be greater than zero")
	}
	return parsed, nil
}

func campaignIDSeed(campaignID uint64) []byte {
	var raw [8]byte
	binary.LittleEndian.PutUint64(raw[:], campaignID)
	return raw[:]
}

func deriveConfigPDA(programID solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress([][]byte{[]byte("config")}, programID)
}

func deriveCampaignPDA(programID, sponsor solana.PublicKey, campaignID uint64) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{
			[]byte("campaign"),
			sponsor.Bytes(),
			campaignIDSeed(campaignID),
		},
		programID,
	)
}

func deriveEscrowPDA(programID, campaignPDA solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{
			[]byte("escrow"),
			campaignPDA.Bytes(),
		},
		programID,
	)
}

func deriveClaimRecordPDA(programID, campaignPDA solana.PublicKey, githubUserID uint64) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{
			[]byte("claim"),
			campaignPDA.Bytes(),
			campaignIDSeed(githubUserID),
		},
		programID,
	)
}

func (c *Client) GetVaultPDA(campaignPDA string) (string, error) {
	campaignKey, err := solana.PublicKeyFromBase58(campaignPDA)
	if err != nil {
		return "", fmt.Errorf("parse campaign PDA: %w", err)
	}

	vaultPDA, _, err := deriveEscrowPDA(c.programID, campaignKey)
	if err != nil {
		return "", fmt.Errorf("derive escrow PDA: %w", err)
	}

	return vaultPDA.String(), nil
}

func (c *Client) GetBalance(ctx context.Context, wallet string) (uint64, error) {
	if !c.IsConfigured() {
		return 0, nil
	}

	walletKey, err := solana.PublicKeyFromBase58(wallet)
	if err != nil {
		return 0, fmt.Errorf("parse wallet pubkey: %w", err)
	}

	balance, err := c.rpcClient.GetBalance(ctx, walletKey, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, fmt.Errorf("get wallet balance: %w", err)
	}

	return uint64(balance.Value), nil
}

func (c *Client) EstimateCreateCampaignCost(ctx context.Context, rewardAmount uint64) (uint64, error) {
	if !c.IsConfigured() {
		return 0, ErrNotConfigured
	}

	serviceFee := rewardAmount * serviceFeeNumerator / serviceFeeDenominator
	if serviceFee < minServiceFeeLamports {
		serviceFee = minServiceFeeLamports
	}

	campaignRent, err := c.rpcClient.GetMinimumBalanceForRentExemption(
		ctx,
		campaignAccountSpace,
		rpc.CommitmentConfirmed,
	)
	if err != nil {
		return 0, fmt.Errorf("get campaign rent exemption: %w", err)
	}

	return rewardAmount + serviceFee + campaignRent, nil
}

type ClaimStatus struct {
	Claimed         bool
	RecipientWallet string
	ClaimedAt       *time.Time
	Amount          uint64
}

func (c *Client) BuildClaimTransaction(
	ctx context.Context,
	campaignID string,
	sponsor string,
	githubUserID uint64,
	userWallet string,
) (string, error) {
	if !c.IsConfigured() {
		return "", ErrNotConfigured
	}

	sponsorKey, err := solana.PublicKeyFromBase58(sponsor)
	if err != nil {
		return "", fmt.Errorf("parse sponsor pubkey: %w", err)
	}

	parsedCampaignID, err := parseCampaignID(campaignID)
	if err != nil {
		return "", err
	}

	userKey, err := solana.PublicKeyFromBase58(userWallet)
	if err != nil {
		return "", fmt.Errorf("parse user wallet: %w", err)
	}

	configPDA, _, err := deriveConfigPDA(c.programID)
	if err != nil {
		return "", fmt.Errorf("derive config PDA: %w", err)
	}

	campaignPDA, _, err := deriveCampaignPDA(c.programID, sponsorKey, parsedCampaignID)
	if err != nil {
		return "", fmt.Errorf("derive campaign PDA: %w", err)
	}

	claimRecordPDA, _, err := deriveClaimRecordPDA(c.programID, campaignPDA, githubUserID)
	if err != nil {
		return "", fmt.Errorf("derive claim record PDA: %w", err)
	}

	escrowPDA, _, err := deriveEscrowPDA(c.programID, campaignPDA)
	if err != nil {
		return "", fmt.Errorf("derive escrow PDA: %w", err)
	}

	recent, err := c.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("get blockhash: %w", err)
	}

	instruction := newClaimInstruction(
		c.programID,
		userKey,
		c.privateKey.PublicKey(),
		configPDA,
		campaignPDA,
		claimRecordPDA,
		escrowPDA,
		userKey,
		githubUserID,
		0,
	)

	tx, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		recent.Value.Blockhash,
		solana.TransactionPayer(userKey),
	)
	if err != nil {
		return "", fmt.Errorf("build transaction: %w", err)
	}

	_, err = tx.PartialSign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(c.privateKey.PublicKey()) {
			return &c.privateKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("partial sign transaction: %w", err)
	}

	txData, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal transaction: %w", err)
	}

	return base58.Encode(txData), nil
}

func (c *Client) GetClaimStatus(
	ctx context.Context,
	campaignID string,
	sponsor string,
	githubUserID uint64,
) (*ClaimStatus, error) {
	if !c.IsConfigured() {
		return nil, ErrNotConfigured
	}

	sponsorKey, err := solana.PublicKeyFromBase58(sponsor)
	if err != nil {
		return nil, fmt.Errorf("parse sponsor pubkey: %w", err)
	}

	parsedCampaignID, err := parseCampaignID(campaignID)
	if err != nil {
		return nil, err
	}

	campaignPDA, _, err := deriveCampaignPDA(c.programID, sponsorKey, parsedCampaignID)
	if err != nil {
		return nil, fmt.Errorf("derive campaign PDA: %w", err)
	}

	claimRecordPDA, _, err := deriveClaimRecordPDA(c.programID, campaignPDA, githubUserID)
	if err != nil {
		return nil, fmt.Errorf("derive claim record PDA: %w", err)
	}

	account, err := c.rpcClient.GetAccountInfo(ctx, claimRecordPDA)
	if err != nil {
		return nil, fmt.Errorf("get claim record: %w", err)
	}
	if account == nil || account.Value == nil {
		return nil, fmt.Errorf("claim record not found")
	}

	status, err := decodeClaimRecordAccount(account.Value.Data.GetBinary())
	if err != nil {
		return nil, err
	}
	return status, nil
}

func newClaimInstruction(
	programID solana.PublicKey,
	user solana.PublicKey,
	claimAuthority solana.PublicKey,
	configPDA solana.PublicKey,
	campaignPDA solana.PublicKey,
	claimRecordPDA solana.PublicKey,
	escrowPDA solana.PublicKey,
	recipientWallet solana.PublicKey,
	githubUserID uint64,
	payerMode uint8,
) solana.Instruction {
	data := anchorDiscriminator("global:claim")
	data = appendBorshU64(data, githubUserID)
	data = append(data, payerMode)

	return solana.NewInstruction(
		programID,
		solana.AccountMetaSlice{
			solana.NewAccountMeta(user, true, true),
			solana.NewAccountMeta(claimAuthority, false, true),
			solana.NewAccountMeta(configPDA, false, false),
			solana.NewAccountMeta(campaignPDA, true, false),
			solana.NewAccountMeta(claimRecordPDA, true, false),
			solana.NewAccountMeta(escrowPDA, true, false),
			solana.NewAccountMeta(recipientWallet, true, false),
			solana.NewAccountMeta(solana.SystemProgramID, false, false),
		},
		data,
	)
}

func (c *Client) ClaimAllocation(ctx context.Context, campaignID, contributorGitHub string, contributorWallet string) (string, error) {
	if !c.IsConfigured() {
		return "", ErrNotConfigured
	}

	campaignPDA, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("campaign"),
			[]byte(campaignID),
		},
		c.programID,
	)
	if err != nil {
		return "", fmt.Errorf("derive campaign PDA: %w", err)
	}

	vaultPDA, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("vault"),
			campaignPDA.Bytes(),
		},
		c.programID,
	)
	if err != nil {
		return "", fmt.Errorf("derive vault PDA: %w", err)
	}

	contributorKey, err := solana.PublicKeyFromBase58(contributorWallet)
	if err != nil {
		return "", fmt.Errorf("parse contributor wallet: %w", err)
	}

	discriminator := anchorDiscriminator("global:claim")

	data := discriminator
	data = appendBorshString(data, contributorGitHub)

	instruction := solana.NewInstruction(
		c.programID,
		solana.AccountMetaSlice{
			solana.NewAccountMeta(campaignPDA, true, false),
			solana.NewAccountMeta(vaultPDA, true, false),
			solana.NewAccountMeta(c.privateKey.PublicKey(), false, true), // authority signer
			solana.NewAccountMeta(contributorKey, true, false),
			solana.NewAccountMeta(solana.SystemProgramID, false, false),
		},
		data,
	)

	return c.sendTransaction(ctx, instruction)
}

type FundTransaction struct {
	Transaction  string `json:"transaction"`
	CampaignPDA  string `json:"campaign_pda"`
	VaultAddress string `json:"vault_address"`
	EscrowPDA    string `json:"escrow_pda,omitempty"`
}

func newCreateCampaignWithDepositInstruction(
	programID solana.PublicKey,
	sponsorKey solana.PublicKey,
	configPDA solana.PublicKey,
	campaignPDA solana.PublicKey,
	escrowPDA solana.PublicKey,
	treasuryWallet solana.PublicKey,
	campaignID uint64,
	githubRepoID uint64,
	deadline int64,
	rewardAmount uint64,
) solana.Instruction {
	data := anchorDiscriminator("global:create_campaign_with_deposit")
	data = appendBorshU64(data, campaignID)
	data = appendBorshU64(data, githubRepoID)
	data = appendBorshI64(data, deadline)
	data = appendBorshU64(data, rewardAmount)

	return solana.NewInstruction(
		programID,
		solana.AccountMetaSlice{
			solana.NewAccountMeta(sponsorKey, true, true),
			solana.NewAccountMeta(configPDA, false, false),
			solana.NewAccountMeta(campaignPDA, true, false),
			solana.NewAccountMeta(escrowPDA, true, false),
			solana.NewAccountMeta(treasuryWallet, true, false),
			solana.NewAccountMeta(solana.SystemProgramID, false, false),
		},
		data,
	)
}

func (c *Client) BuildFundTransaction(
	ctx context.Context,
	campaignID string,
	poolAmount uint64,
	deadline int64,
	githubRepoID uint64,
	sponsorPubkey string,
) (*FundTransaction, error) {
	if !c.IsConfigured() {
		return nil, ErrNotConfigured
	}

	sponsorKey, err := solana.PublicKeyFromBase58(sponsorPubkey)
	if err != nil {
		return nil, fmt.Errorf("parse sponsor pubkey: %w", err)
	}

	parsedCampaignID, err := parseCampaignID(campaignID)
	if err != nil {
		return nil, err
	}

	configPDA, _, err := deriveConfigPDA(c.programID)
	if err != nil {
		return nil, fmt.Errorf("derive config PDA: %w", err)
	}

	campaignPDA, _, err := deriveCampaignPDA(c.programID, sponsorKey, parsedCampaignID)
	if err != nil {
		return nil, fmt.Errorf("derive campaign PDA: %w", err)
	}

	vaultPDA, _, err := deriveEscrowPDA(c.programID, campaignPDA)
	if err != nil {
		return nil, fmt.Errorf("derive escrow PDA: %w", err)
	}

	recent, err := c.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("get blockhash: %w", err)
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{
			newCreateCampaignWithDepositInstruction(
				c.programID,
				sponsorKey,
				configPDA,
				campaignPDA,
				vaultPDA,
				c.privateKey.PublicKey(),
				parsedCampaignID,
				githubRepoID,
				deadline,
				poolAmount,
			),
		},
		recent.Value.Blockhash,
		solana.TransactionPayer(sponsorKey),
	)
	if err != nil {
		return nil, fmt.Errorf("build transaction: %w", err)
	}

	txData, err := tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal transaction: %w", err)
	}

	txBase58 := base58.Encode(txData)

	return &FundTransaction{
		Transaction:  txBase58,
		CampaignPDA:  campaignPDA.String(),
		VaultAddress: vaultPDA.String(),
		EscrowPDA:    vaultPDA.String(),
	}, nil
}

func (c *Client) CreateCampaign(ctx context.Context, campaignID, repo string, poolAmount uint64, deadline int64, sponsorPubkey string) (string, string, string, error) {
	if !c.IsConfigured() {
		return "", "", "", ErrNotConfigured
	}

	authority := c.privateKey.PublicKey()
	campaignPDA, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("campaign"),
			[]byte(campaignID),
		},
		c.programID,
	)
	if err != nil {
		return "", "", "", fmt.Errorf("derive campaign PDA: %w", err)
	}

	vaultPDA, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("vault"),
			campaignPDA.Bytes(),
		},
		c.programID,
	)
	if err != nil {
		return "", "", "", fmt.Errorf("derive vault PDA: %w", err)
	}

	sponsorKey, err := solana.PublicKeyFromBase58(sponsorPubkey)
	if err != nil {
		return "", "", "", fmt.Errorf("parse sponsor pubkey: %w", err)
	}

	discriminator := anchorDiscriminator("global:create_campaign")

	data := discriminator
	data = appendBorshString(data, campaignID)
	data = appendBorshString(data, repo)
	data = appendBorshU64(data, poolAmount)
	data = appendBorshI64(data, deadline)
	data = append(data, sponsorKey.Bytes()...)

	instruction := solana.NewInstruction(
		c.programID,
		solana.AccountMetaSlice{
			solana.NewAccountMeta(campaignPDA, true, false),
			solana.NewAccountMeta(authority, true, true),
			solana.NewAccountMeta(vaultPDA, false, false),
			solana.NewAccountMeta(solana.SystemProgramID, false, false),
		},
		data,
	)

	txSig, err := c.sendTransaction(ctx, instruction)
	if err != nil {
		return "", "", "", err
	}

	return txSig, campaignPDA.String(), vaultPDA.String(), nil
}

type AllocationInput struct {
	GithubUserID uint64
	Amount       uint64
}

func newFinalizeCampaignBatchInstruction(
	programID solana.PublicKey,
	finalizeAuthority solana.PublicKey,
	configPDA solana.PublicKey,
	campaignPDA solana.PublicKey,
	claimRecordPDAs []solana.PublicKey,
	allocations []AllocationInput,
	hasMore bool,
) solana.Instruction {
	data := anchorDiscriminator("global:finalize_campaign_batch")
	data = appendBorshU32(data, uint32(len(allocations)))
	for _, a := range allocations {
		data = appendBorshU64(data, a.GithubUserID)
		data = appendBorshU64(data, a.Amount)
	}
	if hasMore {
		data = append(data, 1)
	} else {
		data = append(data, 0)
	}

	accounts := solana.AccountMetaSlice{
		solana.NewAccountMeta(finalizeAuthority, true, true),
		solana.NewAccountMeta(configPDA, false, false),
		solana.NewAccountMeta(campaignPDA, true, false),
		solana.NewAccountMeta(solana.SystemProgramID, false, false),
	}
	for _, claimRecordPDA := range claimRecordPDAs {
		accounts = append(accounts, solana.NewAccountMeta(claimRecordPDA, true, false))
	}

	return solana.NewInstruction(programID, accounts, data)
}

func (c *Client) FinalizeCampaign(ctx context.Context, campaignID string, sponsor string, allocations []AllocationInput) (string, error) {
	if !c.IsConfigured() {
		return "", ErrNotConfigured
	}

	authority := c.privateKey.PublicKey()
	sponsorKey, err := solana.PublicKeyFromBase58(sponsor)
	if err != nil {
		return "", fmt.Errorf("parse sponsor pubkey: %w", err)
	}

	parsedCampaignID, err := parseCampaignID(campaignID)
	if err != nil {
		return "", err
	}

	configPDA, _, err := deriveConfigPDA(c.programID)
	if err != nil {
		return "", fmt.Errorf("derive config PDA: %w", err)
	}

	campaignPDA, _, err := deriveCampaignPDA(c.programID, sponsorKey, parsedCampaignID)
	if err != nil {
		return "", fmt.Errorf("derive campaign PDA: %w", err)
	}

	claimRecordPDAs := make([]solana.PublicKey, 0, len(allocations))
	for _, allocation := range allocations {
		claimRecordPDA, _, err := deriveClaimRecordPDA(c.programID, campaignPDA, allocation.GithubUserID)
		if err != nil {
			return "", fmt.Errorf("derive claim record PDA: %w", err)
		}
		claimRecordPDAs = append(claimRecordPDAs, claimRecordPDA)
	}

	instruction := newFinalizeCampaignBatchInstruction(
		c.programID,
		authority,
		configPDA,
		campaignPDA,
		claimRecordPDAs,
		allocations,
		false,
	)

	return c.sendTransaction(ctx, instruction)
}

func (c *Client) sendTransaction(ctx context.Context, instruction solana.Instruction) (string, error) {
	recent, err := c.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("get blockhash: %w", err)
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		recent.Value.Blockhash,
		solana.TransactionPayer(c.privateKey.PublicKey()),
	)
	if err != nil {
		return "", fmt.Errorf("build transaction: %w", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(c.privateKey.PublicKey()) {
			return &c.privateKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("sign transaction: %w", err)
	}

	sig, err := c.rpcClient.SendTransaction(ctx, tx)
	if err != nil {
		var rpcErr *rpcjson.RPCError
		if errors.As(err, &rpcErr) {
			dataJSON, marshalErr := json.Marshal(rpcErr.Data)
			if marshalErr == nil {
				return "", fmt.Errorf(
					"send transaction: rpc code=%d message=%s data=%s",
					rpcErr.Code,
					rpcErr.Message,
					string(dataJSON),
				)
			}
			return "", fmt.Errorf(
				"send transaction: rpc code=%d message=%s",
				rpcErr.Code,
				rpcErr.Message,
			)
		}
		return "", fmt.Errorf("send transaction: %w", err)
	}

	log.Printf("solana: transaction sent: %s", sig.String())
	return sig.String(), nil
}

func anchorDiscriminator(namespace string) []byte {
	h := sha256.Sum256([]byte(namespace))
	return h[:8]
}

func decodeCampaignAccount(data []byte, campaignPDA string, programID solana.PublicKey) (*models.Campaign, error) {
	if len(data) < len(campaignAccountDiscriminator) {
		return nil, fmt.Errorf("account too short")
	}
	if !equalBytes(data[:8], campaignAccountDiscriminator) {
		return nil, fmt.Errorf("unexpected discriminator")
	}

	dec := accountDecoder{data: data[8:]}

	if _, err := dec.readU8(); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}

	campaignID, err := dec.readU64()
	if err != nil {
		return nil, fmt.Errorf("read campaign_id: %w", err)
	}

	sponsorBytes, err := dec.readBytes(32)
	if err != nil {
		return nil, fmt.Errorf("read sponsor: %w", err)
	}
	sponsor := solana.PublicKeyFromBytes(sponsorBytes)

	githubRepoID, err := dec.readU64()
	if err != nil {
		return nil, fmt.Errorf("read github_repo_id: %w", err)
	}

	createdAtUnix, err := dec.readI64()
	if err != nil {
		return nil, fmt.Errorf("read created_at: %w", err)
	}

	deadlineUnix, err := dec.readI64()
	if err != nil {
		return nil, fmt.Errorf("read deadline_at: %w", err)
	}

	claimDeadlineUnix, err := dec.readI64()
	if err != nil {
		return nil, fmt.Errorf("read claim_deadline_at: %w", err)
	}

	totalRewardAmount, err := dec.readU64()
	if err != nil {
		return nil, fmt.Errorf("read total_reward_amount: %w", err)
	}

	allocatedAmount, err := dec.readU64()
	if err != nil {
		return nil, fmt.Errorf("read allocated_amount: %w", err)
	}

	claimedAmount, err := dec.readU64()
	if err != nil {
		return nil, fmt.Errorf("read claimed_amount: %w", err)
	}

	allocationsCount, err := dec.readU32()
	if err != nil {
		return nil, fmt.Errorf("read allocations_count: %w", err)
	}

	claimedCount, err := dec.readU32()
	if err != nil {
		return nil, fmt.Errorf("read claimed_count: %w", err)
	}

	statusByte, err := dec.readU8()
	if err != nil {
		return nil, fmt.Errorf("read status: %w", err)
	}

	var compatState models.CampaignState
	var status string
	switch statusByte {
	case 0:
		compatState = models.StateFunded
		status = models.StateActive
	case 1:
		compatState = models.StateFinalized
		status = models.StateFinalized
	case 2:
		compatState = models.StateCompleted
		status = models.StateClosed
	default:
		return nil, fmt.Errorf("unknown campaign status: %d", statusByte)
	}

	if _, err := dec.readU8(); err != nil {
		return nil, fmt.Errorf("read bump: %w", err)
	}
	if _, err := dec.readBytes(64); err != nil {
		return nil, fmt.Errorf("read reserved bytes: %w", err)
	}

	campaignKey, err := solana.PublicKeyFromBase58(campaignPDA)
	if err != nil {
		return nil, fmt.Errorf("parse campaign PDA: %w", err)
	}

	escrowPDA, _, err := deriveEscrowPDA(programID, campaignKey)
	if err != nil {
		return nil, fmt.Errorf("derive escrow PDA: %w", err)
	}

	createdAt := time.Unix(createdAtUnix, 0).UTC()
	deadlineAt := time.Unix(deadlineUnix, 0).UTC()
	claimDeadlineAt := time.Unix(claimDeadlineUnix, 0).UTC()

	return &models.Campaign{
		CampaignID:        strconv.FormatUint(campaignID, 10),
		CampaignPDA:       campaignPDA,
		EscrowPDA:         escrowPDA.String(),
		VaultAddress:      escrowPDA.String(),
		GithubRepoID:      githubRepoID,
		PoolAmount:        totalRewardAmount,
		TotalRewardAmount: totalRewardAmount,
		AllocatedAmount:   allocatedAmount,
		ClaimedAmount:     claimedAmount,
		AllocationsCount:  allocationsCount,
		ClaimedCount:      claimedCount,
		TotalClaimed:      claimedAmount,
		Deadline:          deadlineAt,
		DeadlineAt:        deadlineAt,
		ClaimDeadlineAt:   claimDeadlineAt,
		State:             compatState,
		Status:            status,
		Sponsor:           sponsor.String(),
		Allocations:       nil,
		CreatedAt:         createdAt,
	}, nil
}

func decodeClaimRecordAccount(data []byte) (*ClaimStatus, error) {
	if len(data) < len(claimRecordAccountDiscriminator) {
		return nil, fmt.Errorf("claim record account too short")
	}
	if !equalBytes(data[:8], claimRecordAccountDiscriminator) {
		return nil, fmt.Errorf("unexpected claim record discriminator")
	}

	dec := accountDecoder{data: data[8:]}

	if _, err := dec.readBytes(32); err != nil {
		return nil, fmt.Errorf("read campaign: %w", err)
	}
	if _, err := dec.readU64(); err != nil {
		return nil, fmt.Errorf("read github_user_id: %w", err)
	}
	amount, err := dec.readU64()
	if err != nil {
		return nil, fmt.Errorf("read amount: %w", err)
	}
	claimedRaw, err := dec.readU8()
	if err != nil {
		return nil, fmt.Errorf("read claimed flag: %w", err)
	}
	recipientWallet, err := dec.readOptionPubkey()
	if err != nil {
		return nil, fmt.Errorf("read claimed_to_wallet: %w", err)
	}
	claimedAt, err := dec.readOptionI64()
	if err != nil {
		return nil, fmt.Errorf("read claimed_at: %w", err)
	}

	status := &ClaimStatus{
		Claimed: claimedRaw != 0,
		Amount:  amount,
	}
	if recipientWallet != nil {
		status.RecipientWallet = recipientWallet.String()
	}
	if claimedAt != nil {
		timestamp := time.Unix(*claimedAt, 0).UTC()
		status.ClaimedAt = &timestamp
	}

	return status, nil
}

type accountDecoder struct {
	data []byte
	pos  int
}

func (d *accountDecoder) readBytes(n int) ([]byte, error) {
	if len(d.data)-d.pos < n {
		return nil, fmt.Errorf("unexpected EOF")
	}
	out := d.data[d.pos : d.pos+n]
	d.pos += n
	return out, nil
}

func (d *accountDecoder) readU8() (uint8, error) {
	b, err := d.readBytes(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (d *accountDecoder) readU16() (uint16, error) {
	b, err := d.readBytes(2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(b), nil
}

func (d *accountDecoder) readU32() (uint32, error) {
	b, err := d.readBytes(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (d *accountDecoder) readU64() (uint64, error) {
	b, err := d.readBytes(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b), nil
}

func (d *accountDecoder) readI64() (int64, error) {
	v, err := d.readU64()
	if err != nil {
		return 0, err
	}
	return int64(v), nil
}

func (d *accountDecoder) readString() (string, error) {
	n, err := d.readU32()
	if err != nil {
		return "", err
	}
	b, err := d.readBytes(int(n))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (d *accountDecoder) readOptionPubkey() (*solana.PublicKey, error) {
	tag, err := d.readU8()
	if err != nil {
		return nil, err
	}
	if tag == 0 {
		return nil, nil
	}
	if tag != 1 {
		return nil, fmt.Errorf("invalid option tag %d", tag)
	}
	b, err := d.readBytes(32)
	if err != nil {
		return nil, err
	}
	key := solana.PublicKeyFromBytes(b)
	return &key, nil
}

func (d *accountDecoder) readOptionI64() (*int64, error) {
	tag, err := d.readU8()
	if err != nil {
		return nil, err
	}
	if tag == 0 {
		return nil, nil
	}
	if tag != 1 {
		return nil, fmt.Errorf("invalid option tag %d", tag)
	}
	v, err := d.readI64()
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func appendBorshString(data []byte, s string) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(s)))
	data = append(data, buf...)
	data = append(data, []byte(s)...)
	return data
}

func appendBorshU64(data []byte, v uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	return append(data, buf...)
}

func appendBorshI64(data []byte, v int64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(v))
	return append(data, buf...)
}

func appendBorshU32(data []byte, v uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return append(data, buf...)
}

func appendBorshU16(data []byte, v uint16) []byte {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, v)
	return append(data, buf...)
}
