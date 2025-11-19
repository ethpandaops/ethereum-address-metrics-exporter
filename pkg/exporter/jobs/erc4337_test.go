package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestERC4337_getBalance(t *testing.T) {
	tests := []struct {
		name            string
		address         *AddressERC4337
		balanceResponse string
		wantError       bool
	}{
		{
			name: "successful balance retrieval",
			address: &AddressERC4337{
				Name:     "Account 1",
				Address:  "0x1234567890123456789012345678901234567890",
				Contract: "0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789", // EntryPoint
				Labels:   map[string]string{},
			},
			balanceResponse: "0x0000000000000000000000000000000000000000000000000000000005f5e100", // 100
			wantError:       false,
		},
		{
			name: "zero balance",
			address: &AddressERC4337{
				Name:     "Account 2",
				Address:  "0x0000000000000000000000000000000000000001",
				Contract: "0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789",
				Labels:   map[string]string{},
			},
			balanceResponse: "0x0000000000000000000000000000000000000000000000000000000000000000",
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

			namespace := "erc4337_" + string(rune('a'+i))

			erc4337 := NewERC4337(
				mockClient,
				log,
				15*time.Second,
				namespace,
				map[string]string{},
				[]*AddressERC4337{tt.address},
			)

			err := erc4337.getBalance(tt.address)

			if (err != nil) != tt.wantError {
				t.Errorf("getBalance() error = %v, wantError %v", err, tt.wantError)
			}

			// Verify balanceOf call was made
			if len(mockClient.callLog) != 1 {
				t.Errorf("Expected 1 RPC call (balanceOf), got %d", len(mockClient.callLog))
			}

			if len(mockClient.callLog) > 0 && mockClient.callLog[0].data[:10] != "0x70a08231" {
				t.Errorf("First call should be balanceOf (0x70a08231)")
			}
		})
	}
}

func TestERC4337_tick(t *testing.T) {
	mockClient := &mockExecutionClient{
		balanceOfResponse: "0x0de0b6b3a7640000",
		callLog:           []mockCall{},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressERC4337{
		{
			Name:     "Account 1",
			Address:  "0x1111111111111111111111111111111111111111",
			Contract: "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Labels:   map[string]string{},
		},
		{
			Name:     "Account 2",
			Address:  "0x2222222222222222222222222222222222222222",
			Contract: "0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			Labels:   map[string]string{},
		},
	}

	erc4337 := NewERC4337(
		mockClient,
		log,
		15*time.Second,
		"test_erc4337_tick",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	erc4337.tick(ctx)

	// Each address requires 1 call (balanceOf)
	expectedCalls := len(addresses)
	if len(mockClient.callLog) != expectedCalls {
		t.Errorf("Expected %d RPC calls, got %d", expectedCalls, len(mockClient.callLog))
	}
}

func TestERC4337_getLabelValues(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressERC4337{
		{
			Name:     "Test Account",
			Address:  "0x1234567890123456789012345678901234567890",
			Contract: "0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789",
			Labels: map[string]string{
				"type": "paymaster",
			},
		},
	}

	erc4337 := NewERC4337(
		&mockExecutionClient{},
		log,
		15*time.Second,
		"test_erc4337_labels",
		map[string]string{},
		addresses,
	)

	labels := erc4337.getLabelValues(addresses[0])

	if len(labels) != len(erc4337.labelsMap) {
		t.Errorf("Expected %d label values, got %d", len(erc4337.labelsMap), len(labels))
	}
}

func TestERC4337_Name(t *testing.T) {
	erc4337 := &ERC4337{}
	if erc4337.Name() != NameERC4337 {
		t.Errorf("Expected name %s, got %s", NameERC4337, erc4337.Name())
	}
}
