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
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"
	"github.com/repobounty/repobounty-ai/internal/models"
)

type Client struct {
	rpcClient  *rpc.Client
	privateKey solana.PrivateKey
	programID  solana.PublicKey
}

var campaignAccountDiscriminator = anchorDiscriminator("account:Campaign")

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

	return nil, errors.New("campaign not found")
}

func (c *Client) GetVaultPDA(campaignPDA string) (string, error) {
	campaignKey, err := solana.PublicKeyFromBase58(campaignPDA)
	if err != nil {
		return "", fmt.Errorf("parse campaign PDA: %w", err)
	}

	vaultPDA, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("vault"),
			campaignKey.Bytes(),
		},
		c.programID,
	)
	if err != nil {
		return "", fmt.Errorf("derive vault PDA: %w", err)
	}

	return vaultPDA.String(), nil
}

func (c *Client) ClaimAllocation(ctx context.Context, campaignID, contributorGitHub string, contributorWallet string) (string, error) {
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
}

func (c *Client) BuildFundTransaction(ctx context.Context, campaignID string, poolAmount uint64, sponsorPubkey string) (*FundTransaction, error) {
	if !c.IsConfigured() {
		return &FundTransaction{
			Transaction:  "mock_fund_tx_" + campaignID,
			CampaignPDA:  "mock_campaign_pda",
			VaultAddress: "mock_vault_pda",
		}, nil
	}

	sponsorKey, err := solana.PublicKeyFromBase58(sponsorPubkey)
	if err != nil {
		return nil, fmt.Errorf("parse sponsor pubkey: %w", err)
	}

	campaignPDA, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("campaign"),
			[]byte(campaignID),
		},
		c.programID,
	)
	if err != nil {
		return nil, fmt.Errorf("derive campaign PDA: %w", err)
	}

	vaultPDA, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("vault"),
			campaignPDA.Bytes(),
		},
		c.programID,
	)
	if err != nil {
		return nil, fmt.Errorf("derive vault PDA: %w", err)
	}

	recent, err := c.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("get blockhash: %w", err)
	}

	transferIx, err := system.NewTransferInstruction(
		poolAmount,
		sponsorKey,
		vaultPDA,
	).ValidateAndBuild()
	if err != nil {
		return nil, fmt.Errorf("build transfer instruction: %w", err)
	}

	fundCampaignData := anchorDiscriminator("global:fund_campaign")

	tx, err := solana.NewTransaction(
		[]solana.Instruction{
			transferIx,
			solana.NewInstruction(
				c.programID,
				solana.AccountMetaSlice{
					solana.NewAccountMeta(campaignPDA, true, false),
					solana.NewAccountMeta(vaultPDA, true, false),
					solana.NewAccountMeta(sponsorKey, false, true),
				},
				fundCampaignData,
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
	}, nil
}

func (c *Client) CreateCampaign(ctx context.Context, campaignID, repo string, poolAmount uint64, deadline int64, sponsorPubkey string) (string, string, string, error) {
	if !c.IsConfigured() {
		log.Printf("solana: mock create_campaign for %s", repo)
		return "mock_tx_" + campaignID, "mock_campaign_pda", "mock_vault_pda", nil
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
	Contributor string
	Percentage  uint16
}

func (c *Client) FinalizeCampaign(ctx context.Context, campaignID string, allocations []AllocationInput) (string, error) {
	if !c.IsConfigured() {
		log.Printf("solana: mock finalize_campaign for %s", campaignID)
		return "mock_finalize_tx_" + campaignID, nil
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
		return "", fmt.Errorf("derive campaign PDA: %w", err)
	}

	discriminator := anchorDiscriminator("global:finalize_campaign")

	data := discriminator
	data = appendBorshU32(data, uint32(len(allocations)))
	for _, a := range allocations {
		data = appendBorshString(data, a.Contributor)
		data = appendBorshU16(data, a.Percentage)
	}

	instruction := solana.NewInstruction(
		c.programID,
		solana.AccountMetaSlice{
			solana.NewAccountMeta(campaignPDA, true, false),
			solana.NewAccountMeta(authority, false, true),
		},
		data,
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

	authorityBytes, err := dec.readBytes(32)
	if err != nil {
		return nil, err
	}
	authority := solana.PublicKeyFromBytes(authorityBytes)

	sponsorBytes, err := dec.readBytes(32)
	if err != nil {
		return nil, err
	}
	sponsor := solana.PublicKeyFromBytes(sponsorBytes)

	campaignID, err := dec.readString()
	if err != nil {
		return nil, fmt.Errorf("read campaign_id: %w", err)
	}

	repo, err := dec.readString()
	if err != nil {
		return nil, fmt.Errorf("read repo: %w", err)
	}

	poolAmount, err := dec.readU64()
	if err != nil {
		return nil, fmt.Errorf("read pool_amount: %w", err)
	}

	totalClaimed, err := dec.readU64()
	if err != nil {
		return nil, fmt.Errorf("read total_claimed: %w", err)
	}

	deadlineUnix, err := dec.readI64()
	if err != nil {
		return nil, fmt.Errorf("read deadline: %w", err)
	}

	stateByte, err := dec.readU8()
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}

	var state models.CampaignState
	switch stateByte {
	case 0:
		state = models.StateCreated
	case 1:
		state = models.StateFunded
	case 2:
		state = models.StateFinalized
	case 3:
		state = models.StateCompleted
	default:
		return nil, fmt.Errorf("unknown state: %d", stateByte)
	}

	allocCount, err := dec.readU32()
	if err != nil {
		return nil, fmt.Errorf("read allocations len: %w", err)
	}

	allocations := make([]models.Allocation, 0, allocCount)
	for i := uint32(0); i < allocCount; i++ {
		contributor, err := dec.readString()
		if err != nil {
			return nil, fmt.Errorf("read allocation contributor: %w", err)
		}
		percentage, err := dec.readU16()
		if err != nil {
			return nil, fmt.Errorf("read allocation percentage: %w", err)
		}
		amount, err := dec.readU64()
		if err != nil {
			return nil, fmt.Errorf("read allocation amount: %w", err)
		}
		claimedByte, err := dec.readU8()
		if err != nil {
			return nil, fmt.Errorf("read allocation claimed: %w", err)
		}
		claimed := claimedByte == 1
		claimantWallet := ""
		if claimed {
			claimantTag, err := dec.readU8()
			if err != nil {
				return nil, fmt.Errorf("read claimant tag: %w", err)
			}
			if claimantTag == 1 {
				claimantBytes, err := dec.readBytes(32)
				if err != nil {
					return nil, fmt.Errorf("read claimant pubkey: %w", err)
				}
				claimantKey := solana.PublicKeyFromBytes(claimantBytes)
				claimantWallet = claimantKey.String()
			}
		}
		allocations = append(allocations, models.Allocation{
			Contributor:    contributor,
			Percentage:     percentage,
			Amount:         amount,
			Claimed:        claimed,
			ClaimantWallet: claimantWallet,
		})
	}

	if _, err := dec.readU8(); err != nil {
		return nil, fmt.Errorf("read bump: %w", err)
	}

	createdAtUnix, err := dec.readI64()
	if err != nil {
		return nil, fmt.Errorf("read created_at: %w", err)
	}

	finalizedAtTag, err := dec.readU8()
	if err != nil {
		return nil, fmt.Errorf("read finalized_at tag: %w", err)
	}

	var finalizedAt *time.Time
	if finalizedAtTag == 1 {
		finalizedAtUnix, err := dec.readI64()
		if err != nil {
			return nil, fmt.Errorf("read finalized_at: %w", err)
		}
		t := time.Unix(finalizedAtUnix, 0).UTC()
		finalizedAt = &t
	}

	campaignKey, err := solana.PublicKeyFromBase58(campaignPDA)
	if err != nil {
		return nil, fmt.Errorf("parse campaign PDA: %w", err)
	}

	vaultPDA, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("vault"),
			campaignKey.Bytes(),
		},
		programID,
	)
	if err != nil {
		return nil, fmt.Errorf("derive vault PDA: %w", err)
	}

	return &models.Campaign{
		CampaignID:   campaignID,
		CampaignPDA:  campaignPDA,
		VaultAddress: vaultPDA.String(),
		Repo:         repo,
		PoolAmount:   poolAmount,
		TotalClaimed: totalClaimed,
		Deadline:     time.Unix(deadlineUnix, 0).UTC(),
		State:        state,
		Authority:    authority.String(),
		Sponsor:      sponsor.String(),
		Allocations:  allocations,
		CreatedAt:    time.Unix(createdAtUnix, 0).UTC(),
		FinalizedAt:  finalizedAt,
	}, nil
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
