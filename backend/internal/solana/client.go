package solana

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/binary"
	"fmt"
	"log"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type Client struct {
	rpcClient  *rpc.Client
	privateKey solana.PrivateKey
	programID  solana.PublicKey
}

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

func (c *Client) CreateCampaign(ctx context.Context, campaignID, repo string, poolAmount uint64, deadline int64) (string, error) {
	if !c.IsConfigured() {
		log.Printf("solana: mock create_campaign for %s", repo)
		return "mock_tx_" + campaignID, nil
	}

	authority := c.privateKey.PublicKey()
	campaignPDA, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("campaign"),
			authority.Bytes(),
			[]byte(campaignID),
		},
		c.programID,
	)
	if err != nil {
		return "", fmt.Errorf("derive campaign PDA: %w", err)
	}

	discriminator := anchorDiscriminator("global:create_campaign")

	data := discriminator
	data = appendBorshString(data, campaignID)
	data = appendBorshString(data, repo)
	data = appendBorshU64(data, poolAmount)
	data = appendBorshI64(data, deadline)

	instruction := solana.NewInstruction(
		c.programID,
		solana.AccountMetaSlice{
			solana.NewAccountMeta(campaignPDA, false, true),
			solana.NewAccountMeta(authority, true, true),
			solana.NewAccountMeta(solana.SystemProgramID, false, false),
		},
		data,
	)

	return c.sendTransaction(ctx, instruction)
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
			authority.Bytes(),
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
			solana.NewAccountMeta(campaignPDA, false, true),
			solana.NewAccountMeta(authority, true, false),
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
