package solana

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	gosolana "github.com/gagliardetto/solana-go"
)

func TestDecodeCampaignAccountHandlesCurrentCampaignLayout(t *testing.T) {
	programID, err := gosolana.PublicKeyFromBase58("97t3t188wnRoogkD8SoZKWaWbP9qDdN9gUwS4Bdw7Qdo")
	if err != nil {
		t.Fatalf("parse program id: %v", err)
	}

	campaignKey := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{9}, 32))
	authority := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{1}, 32))
	sponsor := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{2}, 32))
	claimant := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{3}, 32))

	const (
		poolAmount   = uint64(1_000_000_000)
		totalClaimed = uint64(500_000_000)
		deadlineUnix = int64(1_700_000_000)
		createdAt    = int64(1_700_000_100)
		finalizedAt  = int64(1_700_000_200)
	)

	data := append([]byte{}, anchorDiscriminator("account:Campaign")...)
	data = append(data, authority[:]...)
	data = append(data, sponsor[:]...)
	data = appendTestString(data, "camp-123")
	data = appendTestString(data, "owner/repo")
	data = appendTestU64(data, poolAmount)
	data = appendTestU64(data, totalClaimed)
	data = appendTestI64(data, deadlineUnix)
	data = append(data, byte(1)) // Funded

	data = appendTestU32(data, 2)

	data = appendTestString(data, "alice")
	data = appendTestU16(data, 5000)
	data = appendTestU64(data, 500_000_000)
	data = append(data, byte(0)) // claimed = false
	data = append(data, byte(0)) // claimant = None

	data = appendTestString(data, "bob")
	data = appendTestU16(data, 5000)
	data = appendTestU64(data, 500_000_000)
	data = append(data, byte(1)) // claimed = true
	data = append(data, byte(1)) // claimant = Some
	data = append(data, claimant[:]...)

	data = append(data, byte(254)) // bump
	data = append(data, byte(253)) // vault bump
	data = appendTestI64(data, createdAt)
	data = append(data, byte(1)) // finalized_at = Some
	data = appendTestI64(data, finalizedAt)
	data = append(data, byte(0)) // campaign_type = Deadline
	data = appendTestU32(data, 0)
	data = appendTestU32(data, 0)

	campaign, err := decodeCampaignAccount(data, campaignKey.String(), programID)
	if err != nil {
		t.Fatalf("decode campaign account: %v", err)
	}

	if campaign.CampaignID != "camp-123" {
		t.Fatalf("unexpected campaign id: %s", campaign.CampaignID)
	}
	if campaign.State != "funded" {
		t.Fatalf("unexpected campaign state: %s", campaign.State)
	}
	if campaign.TotalClaimed != totalClaimed {
		t.Fatalf("unexpected total claimed: %d", campaign.TotalClaimed)
	}
	if got := campaign.CreatedAt.UTC(); !got.Equal(time.Unix(createdAt, 0).UTC()) {
		t.Fatalf("unexpected created_at: %s", got)
	}
	if campaign.FinalizedAt == nil || !campaign.FinalizedAt.UTC().Equal(time.Unix(finalizedAt, 0).UTC()) {
		t.Fatalf("unexpected finalized_at: %v", campaign.FinalizedAt)
	}
	if len(campaign.Allocations) != 2 {
		t.Fatalf("unexpected allocations len: %d", len(campaign.Allocations))
	}
	if campaign.Allocations[0].Contributor != "alice" || campaign.Allocations[0].Claimed {
		t.Fatalf("unexpected first allocation: %+v", campaign.Allocations[0])
	}
	if campaign.Allocations[1].Contributor != "bob" || !campaign.Allocations[1].Claimed {
		t.Fatalf("unexpected second allocation: %+v", campaign.Allocations[1])
	}
	if campaign.Allocations[1].ClaimantWallet != claimant.String() {
		t.Fatalf("unexpected claimant wallet: %s", campaign.Allocations[1].ClaimantWallet)
	}

	expectedVault, _, err := gosolana.FindProgramAddress(
		[][]byte{
			[]byte("vault"),
			campaignKey.Bytes(),
		},
		programID,
	)
	if err != nil {
		t.Fatalf("derive vault pda: %v", err)
	}
	if campaign.VaultAddress != expectedVault.String() {
		t.Fatalf("unexpected vault address: %s", campaign.VaultAddress)
	}
}

func TestNewFundCampaignInstructionIncludesSystemProgram(t *testing.T) {
	programID, err := gosolana.PublicKeyFromBase58("97t3t188wnRoogkD8SoZKWaWbP9qDdN9gUwS4Bdw7Qdo")
	if err != nil {
		t.Fatalf("parse program id: %v", err)
	}

	campaignKey := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{4}, 32))
	vaultKey := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{5}, 32))
	sponsorKey := gosolana.PublicKeyFromBytes(bytes.Repeat([]byte{6}, 32))

	instruction := newFundCampaignInstruction(programID, campaignKey, vaultKey, sponsorKey)
	accounts := instruction.Accounts()
	if len(accounts) != 4 {
		t.Fatalf("unexpected account metas length: %d", len(accounts))
	}
	if accounts[3].PublicKey != gosolana.SystemProgramID {
		t.Fatalf("expected system program account, got %s", accounts[3].PublicKey.String())
	}
	if !accounts[2].IsSigner {
		t.Fatal("expected sponsor account to be signer")
	}
	if !accounts[0].IsWritable || !accounts[1].IsWritable {
		t.Fatal("expected campaign and vault accounts to be writable")
	}
}

func appendTestString(data []byte, value string) []byte {
	data = appendTestU32(data, uint32(len(value)))
	return append(data, []byte(value)...)
}

func appendTestU16(data []byte, value uint16) []byte {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, value)
	return append(data, buf...)
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
