package solana

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	gosolana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
	"github.com/repobounty/repobounty-ai/internal/models"
)

func TestDecodeCampaignAccountHandlesV2CampaignLayout(t *testing.T) {
	programID, err := gosolana.PublicKeyFromBase58("5VdatUgJ6AsZ7RbC8TBz6AxUdBNtQ6MckLsKbxgZQdS6")
	if err != nil {
		t.Fatalf("parse program id: %v", err)
	}

	campaignKey := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{9}, 32))
	sponsor := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{2}, 32))

	const (
		campaignID        = uint64(42)
		githubRepoID      = uint64(123456)
		totalRewardAmount = uint64(1_000_000_000)
		allocatedAmount   = uint64(600_000_000)
		claimedAmount     = uint64(200_000_000)
		createdAtUnix     = int64(1_700_000_100)
		deadlineUnix      = int64(1_700_086_400)
		claimDeadlineUnix = int64(1_731_622_400)
	)

	data := append([]byte{}, anchorDiscriminator("account:Campaign")...)
	data = append(data, byte(1)) // version
	data = appendTestU64(data, campaignID)
	data = append(data, sponsor[:]...)
	data = appendTestU64(data, githubRepoID)
	data = appendTestI64(data, createdAtUnix)
	data = appendTestI64(data, deadlineUnix)
	data = appendTestI64(data, claimDeadlineUnix)
	data = appendTestU64(data, totalRewardAmount)
	data = appendTestU64(data, allocatedAmount)
	data = appendTestU64(data, claimedAmount)
	data = appendTestU32(data, 3) // allocations_count
	data = appendTestU32(data, 1) // claimed_count
	data = append(data, byte(0))  // active
	data = append(data, byte(254))
	data = append(data, bytes.Repeat([]byte{0}, 64)...)

	campaign, err := decodeCampaignAccount(data, campaignKey.String(), programID)
	if err != nil {
		t.Fatalf("decode campaign account: %v", err)
	}

	if campaign.CampaignID != "42" {
		t.Fatalf("unexpected campaign id: %s", campaign.CampaignID)
	}
	if campaign.GithubRepoID != githubRepoID {
		t.Fatalf("unexpected github repo id: %d", campaign.GithubRepoID)
	}
	if campaign.State != models.StateFunded {
		t.Fatalf("unexpected compat state: %s", campaign.State)
	}
	if campaign.Status != models.StateActive {
		t.Fatalf("unexpected status: %s", campaign.Status)
	}
	if campaign.PoolAmount != totalRewardAmount || campaign.TotalRewardAmount != totalRewardAmount {
		t.Fatalf("unexpected reward amount: pool=%d total=%d", campaign.PoolAmount, campaign.TotalRewardAmount)
	}
	if campaign.AllocatedAmount != allocatedAmount {
		t.Fatalf("unexpected allocated amount: %d", campaign.AllocatedAmount)
	}
	if campaign.ClaimedAmount != claimedAmount || campaign.TotalClaimed != claimedAmount {
		t.Fatalf("unexpected claimed amount: claimed=%d total_claimed=%d", campaign.ClaimedAmount, campaign.TotalClaimed)
	}
	if campaign.AllocationsCount != 3 || campaign.ClaimedCount != 1 {
		t.Fatalf("unexpected counters: allocations=%d claimed=%d", campaign.AllocationsCount, campaign.ClaimedCount)
	}
	if campaign.Sponsor != sponsor.String() {
		t.Fatalf("unexpected sponsor: %s", campaign.Sponsor)
	}
	if got := campaign.CreatedAt.UTC(); !got.Equal(time.Unix(createdAtUnix, 0).UTC()) {
		t.Fatalf("unexpected created_at: %s", got)
	}
	if got := campaign.Deadline.UTC(); !got.Equal(time.Unix(deadlineUnix, 0).UTC()) {
		t.Fatalf("unexpected deadline: %s", got)
	}
	if got := campaign.ClaimDeadlineAt.UTC(); !got.Equal(time.Unix(claimDeadlineUnix, 0).UTC()) {
		t.Fatalf("unexpected claim deadline: %s", got)
	}

	expectedEscrow, _, err := gosolana.FindProgramAddress(
		[][]byte{
			[]byte("escrow"),
			campaignKey.Bytes(),
		},
		programID,
	)
	if err != nil {
		t.Fatalf("derive escrow pda: %v", err)
	}
	if campaign.EscrowPDA != expectedEscrow.String() {
		t.Fatalf("unexpected escrow pda: %s", campaign.EscrowPDA)
	}
	if campaign.VaultAddress != expectedEscrow.String() {
		t.Fatalf("unexpected vault compatibility alias: %s", campaign.VaultAddress)
	}
}

func TestNewCreateCampaignWithDepositInstructionIncludesV2Accounts(t *testing.T) {
	programID, err := gosolana.PublicKeyFromBase58("5VdatUgJ6AsZ7RbC8TBz6AxUdBNtQ6MckLsKbxgZQdS6")
	if err != nil {
		t.Fatalf("parse program id: %v", err)
	}

	sponsor := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{1}, 32))
	config := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{2}, 32))
	campaign := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{3}, 32))
	escrow := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{4}, 32))
	treasury := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{5}, 32))

	instruction := newCreateCampaignWithDepositInstruction(
		programID,
		sponsor,
		config,
		campaign,
		escrow,
		treasury,
		42,
		123456,
		1_700_086_400,
		1_000_000_000,
	)
	accounts := instruction.Accounts()
	if len(accounts) != 6 {
		t.Fatalf("unexpected account metas length: %d", len(accounts))
	}
	if accounts[0].PublicKey != sponsor || !accounts[0].IsSigner || !accounts[0].IsWritable {
		t.Fatalf("unexpected sponsor meta: %+v", accounts[0])
	}
	if accounts[1].PublicKey != config || accounts[1].IsSigner || accounts[1].IsWritable {
		t.Fatalf("unexpected config meta: %+v", accounts[1])
	}
	if accounts[2].PublicKey != campaign || accounts[2].IsSigner || !accounts[2].IsWritable {
		t.Fatalf("unexpected campaign meta: %+v", accounts[2])
	}
	if accounts[3].PublicKey != escrow || accounts[3].IsSigner || !accounts[3].IsWritable {
		t.Fatalf("unexpected escrow meta: %+v", accounts[3])
	}
	if accounts[4].PublicKey != treasury || accounts[4].IsSigner || !accounts[4].IsWritable {
		t.Fatalf("unexpected treasury meta: %+v", accounts[4])
	}
	if accounts[5].PublicKey != gosolana.SystemProgramID || accounts[5].IsSigner || accounts[5].IsWritable {
		t.Fatalf("unexpected system program meta: %+v", accounts[5])
	}
}

func TestNewFinalizeCampaignBatchInstructionUsesV2PayloadAndAccounts(t *testing.T) {
	programID, err := gosolana.PublicKeyFromBase58("5VdatUgJ6AsZ7RbC8TBz6AxUdBNtQ6MckLsKbxgZQdS6")
	if err != nil {
		t.Fatalf("parse program id: %v", err)
	}

	finalizeAuthority := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{1}, 32))
	config := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{2}, 32))
	campaign := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{3}, 32))
	claimOne := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{4}, 32))
	claimTwo := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{5}, 32))

	allocations := []AllocationInput{
		{GithubUserID: 101, Amount: 700_000_000},
		{GithubUserID: 202, Amount: 300_000_000},
	}

	instruction := newFinalizeCampaignBatchInstruction(
		programID,
		finalizeAuthority,
		config,
		campaign,
		[]gosolana.PublicKey{claimOne, claimTwo},
		allocations,
		false,
	)

	accounts := instruction.Accounts()
	if len(accounts) != 6 {
		t.Fatalf("unexpected account metas length: %d", len(accounts))
	}
	if accounts[0].PublicKey != finalizeAuthority || !accounts[0].IsSigner || !accounts[0].IsWritable {
		t.Fatalf("unexpected finalize authority meta: %+v", accounts[0])
	}
	if accounts[1].PublicKey != config || accounts[1].IsSigner || accounts[1].IsWritable {
		t.Fatalf("unexpected config meta: %+v", accounts[1])
	}
	if accounts[2].PublicKey != campaign || accounts[2].IsSigner || !accounts[2].IsWritable {
		t.Fatalf("unexpected campaign meta: %+v", accounts[2])
	}
	if accounts[3].PublicKey != gosolana.SystemProgramID || accounts[3].IsSigner || accounts[3].IsWritable {
		t.Fatalf("unexpected system program meta: %+v", accounts[3])
	}
	if accounts[4].PublicKey != claimOne || accounts[4].IsSigner || !accounts[4].IsWritable {
		t.Fatalf("unexpected first remaining claim record meta: %+v", accounts[4])
	}
	if accounts[5].PublicKey != claimTwo || accounts[5].IsSigner || !accounts[5].IsWritable {
		t.Fatalf("unexpected second remaining claim record meta: %+v", accounts[5])
	}

	data, err := instruction.Data()
	if err != nil {
		t.Fatalf("instruction data: %v", err)
	}
	if !equalBytes(data[:8], anchorDiscriminator("global:finalize_campaign_batch")) {
		t.Fatalf("unexpected discriminator: %x", data[:8])
	}

	dec := accountDecoder{data: data[8:]}
	count, err := dec.readU32()
	if err != nil {
		t.Fatalf("read allocation count: %v", err)
	}
	if count != 2 {
		t.Fatalf("allocation count = %d, want 2", count)
	}

	firstUserID, err := dec.readU64()
	if err != nil {
		t.Fatalf("read first github user id: %v", err)
	}
	firstAmount, err := dec.readU64()
	if err != nil {
		t.Fatalf("read first amount: %v", err)
	}
	secondUserID, err := dec.readU64()
	if err != nil {
		t.Fatalf("read second github user id: %v", err)
	}
	secondAmount, err := dec.readU64()
	if err != nil {
		t.Fatalf("read second amount: %v", err)
	}
	hasMore, err := dec.readU8()
	if err != nil {
		t.Fatalf("read has_more flag: %v", err)
	}

	if firstUserID != 101 || firstAmount != 700_000_000 {
		t.Fatalf("unexpected first allocation payload: user=%d amount=%d", firstUserID, firstAmount)
	}
	if secondUserID != 202 || secondAmount != 300_000_000 {
		t.Fatalf("unexpected second allocation payload: user=%d amount=%d", secondUserID, secondAmount)
	}
	if hasMore != 0 {
		t.Fatalf("unexpected has_more flag: %d", hasMore)
	}
}

func TestNewClaimInstructionUsesV2PayloadAndAccounts(t *testing.T) {
	programID, err := gosolana.PublicKeyFromBase58("5VdatUgJ6AsZ7RbC8TBz6AxUdBNtQ6MckLsKbxgZQdS6")
	if err != nil {
		t.Fatalf("parse program id: %v", err)
	}

	user := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{1}, 32))
	claimAuthority := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{2}, 32))
	config := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{3}, 32))
	campaign := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{4}, 32))
	claimRecord := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{5}, 32))
	escrow := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{6}, 32))

	instruction := newClaimInstruction(
		programID,
		user,
		claimAuthority,
		config,
		campaign,
		claimRecord,
		escrow,
		user,
		9001,
		0,
	)

	accounts := instruction.Accounts()
	if len(accounts) != 8 {
		t.Fatalf("unexpected account metas length: %d", len(accounts))
	}
	if accounts[0].PublicKey != user || !accounts[0].IsSigner || !accounts[0].IsWritable {
		t.Fatalf("unexpected user meta: %+v", accounts[0])
	}
	if accounts[1].PublicKey != claimAuthority || !accounts[1].IsSigner || accounts[1].IsWritable {
		t.Fatalf("unexpected claim authority meta: %+v", accounts[1])
	}
	if accounts[2].PublicKey != config || accounts[2].IsSigner || accounts[2].IsWritable {
		t.Fatalf("unexpected config meta: %+v", accounts[2])
	}
	if accounts[3].PublicKey != campaign || accounts[3].IsSigner || !accounts[3].IsWritable {
		t.Fatalf("unexpected campaign meta: %+v", accounts[3])
	}
	if accounts[4].PublicKey != claimRecord || accounts[4].IsSigner || !accounts[4].IsWritable {
		t.Fatalf("unexpected claim record meta: %+v", accounts[4])
	}
	if accounts[5].PublicKey != escrow || accounts[5].IsSigner || !accounts[5].IsWritable {
		t.Fatalf("unexpected escrow meta: %+v", accounts[5])
	}
	if accounts[6].PublicKey != user || accounts[6].IsSigner || !accounts[6].IsWritable {
		t.Fatalf("unexpected recipient wallet meta: %+v", accounts[6])
	}
	if accounts[7].PublicKey != gosolana.SystemProgramID || accounts[7].IsSigner || accounts[7].IsWritable {
		t.Fatalf("unexpected system program meta: %+v", accounts[7])
	}

	data, err := instruction.Data()
	if err != nil {
		t.Fatalf("instruction data: %v", err)
	}
	if !equalBytes(data[:8], anchorDiscriminator("global:claim")) {
		t.Fatalf("unexpected discriminator: %x", data[:8])
	}

	dec := accountDecoder{data: data[8:]}
	githubUserID, err := dec.readU64()
	if err != nil {
		t.Fatalf("read github user id: %v", err)
	}
	payerMode, err := dec.readU8()
	if err != nil {
		t.Fatalf("read payer mode: %v", err)
	}

	if githubUserID != 9001 {
		t.Fatalf("github user id = %d, want 9001", githubUserID)
	}
	if payerMode != 0 {
		t.Fatalf("payer mode = %d, want 0", payerMode)
	}
}

func TestNewRefundUnclaimedInstructionUsesV2Accounts(t *testing.T) {
	programID, err := gosolana.PublicKeyFromBase58("5VdatUgJ6AsZ7RbC8TBz6AxUdBNtQ6MckLsKbxgZQdS6")
	if err != nil {
		t.Fatalf("parse program id: %v", err)
	}

	sponsor := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{1}, 32))
	config := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{2}, 32))
	campaign := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{3}, 32))
	escrow := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{4}, 32))

	instruction := newRefundUnclaimedInstruction(programID, sponsor, config, campaign, escrow)
	accounts := instruction.Accounts()

	if len(accounts) != 5 {
		t.Fatalf("len(accounts) = %d, want 5", len(accounts))
	}
	if accounts[0].PublicKey != sponsor || !accounts[0].IsSigner || !accounts[0].IsWritable {
		t.Fatalf("unexpected sponsor meta: %+v", accounts[0])
	}
	if accounts[1].PublicKey != config || accounts[1].IsSigner || accounts[1].IsWritable {
		t.Fatalf("unexpected config meta: %+v", accounts[1])
	}
	if accounts[2].PublicKey != campaign || accounts[2].IsSigner || !accounts[2].IsWritable {
		t.Fatalf("unexpected campaign meta: %+v", accounts[2])
	}
	if accounts[3].PublicKey != escrow || accounts[3].IsSigner || !accounts[3].IsWritable {
		t.Fatalf("unexpected escrow meta: %+v", accounts[3])
	}
}

func TestVerifyRefundTransactionAcceptsMatchingRefundTransaction(t *testing.T) {
	programID, err := gosolana.PublicKeyFromBase58("5VdatUgJ6AsZ7RbC8TBz6AxUdBNtQ6MckLsKbxgZQdS6")
	if err != nil {
		t.Fatalf("parse program id: %v", err)
	}

	sponsor := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{1}, 32))
	tx := mustRefundTransaction(t, programID, sponsor, 42)

	client := &Client{
		rpcClient: rpc.NewWithCustomRPCClient(&refundTransactionRPCMock{
			t:       t,
			payload: refundTransactionRPCPayload(t, tx, nil),
		}),
		programID: programID,
	}

	if err := client.VerifyRefundTransaction(
		context.Background(),
		"42",
		sponsor.String(),
		gosolana.MustSignatureFromBase58("5yUSwqQqeZLEEYKxnG4JC4XhaaBpV3RS4nQbK8bQTyjLX5btVq9A1Ja5nuJzV7Z3Zq8G6EVKFvN4DKUL6PSAxmTk").String(),
	); err != nil {
		t.Fatalf("VerifyRefundTransaction: %v", err)
	}
}

func TestVerifyRefundTransactionRejectsWrongCampaign(t *testing.T) {
	programID, err := gosolana.PublicKeyFromBase58("5VdatUgJ6AsZ7RbC8TBz6AxUdBNtQ6MckLsKbxgZQdS6")
	if err != nil {
		t.Fatalf("parse program id: %v", err)
	}

	sponsor := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{1}, 32))
	tx := mustRefundTransaction(t, programID, sponsor, 43)

	client := &Client{
		rpcClient: rpc.NewWithCustomRPCClient(&refundTransactionRPCMock{
			t:       t,
			payload: refundTransactionRPCPayload(t, tx, nil),
		}),
		programID: programID,
	}

	if err := client.VerifyRefundTransaction(
		context.Background(),
		"42",
		sponsor.String(),
		gosolana.MustSignatureFromBase58("5yUSwqQqeZLEEYKxnG4JC4XhaaBpV3RS4nQbK8bQTyjLX5btVq9A1Ja5nuJzV7Z3Zq8G6EVKFvN4DKUL6PSAxmTk").String(),
	); err == nil {
		t.Fatal("VerifyRefundTransaction succeeded for a refund transaction from a different campaign")
	}
}

func appendTestU32(data []byte, value uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, value)
	return append(data, buf...)
}

func appendTestU64(data []byte, value uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, value)
	return append(data, buf...)
}

func appendTestI64(data []byte, value int64) []byte {
	return appendTestU64(data, uint64(value))
}

func mustRefundTransaction(t *testing.T, programID gosolana.PublicKey, sponsor gosolana.PublicKey, campaignID uint64) *gosolana.Transaction {
	t.Helper()

	configPDA, _, err := deriveConfigPDA(programID)
	if err != nil {
		t.Fatalf("derive config PDA: %v", err)
	}

	campaignPDA, _, err := deriveCampaignPDA(programID, sponsor, campaignID)
	if err != nil {
		t.Fatalf("derive campaign PDA: %v", err)
	}

	escrowPDA, _, err := deriveEscrowPDA(programID, campaignPDA)
	if err != nil {
		t.Fatalf("derive escrow PDA: %v", err)
	}

	tx, err := gosolana.NewTransaction(
		[]gosolana.Instruction{
			newRefundUnclaimedInstruction(programID, sponsor, configPDA, campaignPDA, escrowPDA),
		},
		gosolana.HashFromBytes(bytes.Repeat([]byte{7}, 32)),
		gosolana.TransactionPayer(sponsor),
	)
	if err != nil {
		t.Fatalf("new transaction: %v", err)
	}

	return tx
}

type refundTransactionRPCMock struct {
	t       *testing.T
	payload []byte
}

func (m *refundTransactionRPCMock) CallForInto(ctx context.Context, out interface{}, method string, params []interface{}) error {
	if method != "getTransaction" {
		m.t.Fatalf("rpc method = %q, want %q", method, "getTransaction")
	}
	if len(params) != 2 {
		m.t.Fatalf("rpc params length = %d, want 2", len(params))
	}
	return json.Unmarshal(m.payload, out)
}

func (*refundTransactionRPCMock) CallWithCallback(ctx context.Context, method string, params []interface{}, callback func(*http.Request, *http.Response) error) error {
	return nil
}

func (*refundTransactionRPCMock) CallBatch(ctx context.Context, requests jsonrpc.RPCRequests) (jsonrpc.RPCResponses, error) {
	return nil, nil
}

func refundTransactionRPCPayload(t *testing.T, tx *gosolana.Transaction, metaErr any) []byte {
	t.Helper()

	txJSON, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("marshal tx: %v", err)
	}

	payload, err := json.Marshal(map[string]any{
		"slot":        1,
		"blockTime":   123,
		"transaction": json.RawMessage(txJSON),
		"meta": map[string]any{
			"err":               metaErr,
			"fee":               5000,
			"preBalances":       []uint64{},
			"postBalances":      []uint64{},
			"innerInstructions": []any{},
			"preTokenBalances":  []any{},
			"postTokenBalances": []any{},
			"logMessages":       []string{},
			"status":            map[string]any{"Ok": nil},
			"rewards":           []any{},
			"loadedAddresses": map[string]any{
				"readonly": []string{},
				"writable": []string{},
			},
		},
		"version": "legacy",
	})
	if err != nil {
		t.Fatalf("marshal rpc payload: %v", err)
	}
	return payload
}
