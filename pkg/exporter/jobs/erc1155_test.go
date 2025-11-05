package jobs

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestERC1155_getBalance(t *testing.T) {
	tokenID := big.NewInt(100)

	tests := []struct {
		name            string
		address         *AddressERC1155
		balanceResponse string
		wantError       bool
	}{
		{
			name: "successful ERC1155 balance retrieval",
			address: &AddressERC1155{
				Name:     "Gaming NFT",
				Address:  "0x1234567890123456789012345678901234567890",
				Contract: "0x76BE3b62873462d2142405439777e971754E8E77",
				TokenID:  *tokenID,
				Labels:   map[string]string{"type": "gaming"},
			},
			balanceResponse: "0x000000000000000000000000000000000000000000000000000000000000000a", // 10 tokens
			wantError:       false,
		},
		{
			name: "zero balance",
			address: &AddressERC1155{
				Name:     "Empty Wallet",
				Address:  "0x0000000000000000000000000000000000000001",
				Contract: "0x76BE3b62873462d2142405439777e971754E8E77",
				TokenID:  *tokenID,
				Labels:   map[string]string{},
			},
			balanceResponse: "0x0000000000000000000000000000000000000000000000000000000000000000",
			wantError:       false,
		},
		{
			name: "large balance",
			address: &AddressERC1155{
				Name:     "Whale Wallet",
				Address:  "0x0000000000000000000000000000000000000002",
				Contract: "0x76BE3b62873462d2142405439777e971754E8E77",
				TokenID:  *tokenID,
				Labels:   map[string]string{},
			},
			balanceResponse: "0x00000000000000000000000000000000000000000000000000000000000003e8", // 1000 tokens
			wantError:       false,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockExecutionClient{
				balanceOfResponse: tt.balanceResponse,
			}

			log := logrus.New()
			log.SetLevel(logrus.ErrorLevel)

			namespace := "erc1155_" + string(rune('a'+i))

			erc1155 := NewERC1155(
				mockClient,
				log,
				15*time.Second,
				namespace,
				map[string]string{},
				[]*AddressERC1155{tt.address},
			)

			err := erc1155.getBalance(tt.address)

			if (err != nil) != tt.wantError {
				t.Errorf("getBalance() error = %v, wantError %v", err, tt.wantError)
			}

			// Verify balanceOf call was made
			if len(mockClient.callLog) != 1 {
				t.Errorf("Expected 1 RPC call (balanceOf), got %d", len(mockClient.callLog))
			}

			if len(mockClient.callLog) > 0 && mockClient.callLog[0].data[:10] != "0x00fdd58e" {
				t.Errorf("Call should be balanceOf for ERC1155 (0x00fdd58e), got %s", mockClient.callLog[0].data[:10])
			}
		})
	}
}

func TestERC1155_tick(t *testing.T) {
	tokenID := big.NewInt(100)

	mockClient := &mockExecutionClient{
		balanceOfResponse: "0x0000000000000000000000000000000000000000000000000000000000000005",
		callLog:           []mockCall{},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressERC1155{
		{
			Name:     "Token 1",
			Address:  "0x1111111111111111111111111111111111111111",
			Contract: "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			TokenID:  *tokenID,
			Labels:   map[string]string{},
		},
		{
			Name:     "Token 2",
			Address:  "0x2222222222222222222222222222222222222222",
			Contract: "0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			TokenID:  *tokenID,
			Labels:   map[string]string{},
		},
	}

	erc1155 := NewERC1155(
		mockClient,
		log,
		15*time.Second,
		"test_erc1155_tick",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	erc1155.tick(ctx)

	// Each address requires 1 call (balanceOf)
	expectedCalls := len(addresses)
	if len(mockClient.callLog) != expectedCalls {
		t.Errorf("Expected %d RPC calls, got %d", expectedCalls, len(mockClient.callLog))
	}
}

func TestERC1155_getLabelValues(t *testing.T) {
	tokenID := big.NewInt(100)

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressERC1155{
		{
			Name:     "Test ERC1155",
			Address:  "0x1234567890123456789012345678901234567890",
			Contract: "0x76BE3b62873462d2142405439777e971754E8E77",
			TokenID:  *tokenID,
			Labels: map[string]string{
				"type": "gaming",
			},
		},
	}

	erc1155 := NewERC1155(
		&mockExecutionClient{},
		log,
		15*time.Second,
		"test_erc1155_labels",
		map[string]string{},
		addresses,
	)

	labels := erc1155.getLabelValues(addresses[0])

	if len(labels) != len(erc1155.labelsMap) {
		t.Errorf("Expected %d label values, got %d", len(erc1155.labelsMap), len(labels))
	}

	hasTokenID := false

	for _, label := range labels {
		if label == tokenID.String() {
			hasTokenID = true
		}
	}

	if !hasTokenID {
		t.Error("Label values should contain the token ID")
	}
}

func TestERC1155_Name(t *testing.T) {
	erc1155 := &ERC1155{}
	if erc1155.Name() != NameERC1155 {
		t.Errorf("Expected name %s, got %s", NameERC1155, erc1155.Name())
	}
}
