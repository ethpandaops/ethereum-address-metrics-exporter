package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestERC20_getBalance(t *testing.T) {
	tests := []struct {
		name            string
		address         *AddressERC20
		balanceResponse string
		symbolResponse  string
		wantError       bool
	}{
		{
			name: "successful balance retrieval with symbol",
			address: &AddressERC20{
				Name:     "USDC Balance",
				Address:  "0x1234567890123456789012345678901234567890",
				Contract: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
				Labels:   map[string]string{"type": "stablecoin"},
			},
			balanceResponse: "0x0000000000000000000000000000000000000000000000000000000005f5e100", // 100 USDC (6 decimals)
			symbolResponse:  "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000455534443000000000000000000000000000000000000000000000000000000",
			wantError:       false,
		},
		{
			name: "zero balance",
			address: &AddressERC20{
				Name:     "Empty Wallet",
				Address:  "0x0000000000000000000000000000000000000001",
				Contract: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
				Labels:   map[string]string{},
			},
			balanceResponse: "0x0000000000000000000000000000000000000000000000000000000000000000",
			symbolResponse:  "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000455534443000000000000000000000000000000000000000000000000000000",
			wantError:       false,
		},
		{
			name: "large balance",
			address: &AddressERC20{
				Name:     "Whale Wallet",
				Address:  "0x0000000000000000000000000000000000000002",
				Contract: "0x6B175474E89094C44Da98b954EedeAC495271d0F",
				Labels:   map[string]string{},
			},
			balanceResponse: "0x0000000000000000000000000000000000000000000000056bc75e2d63100000", // Large amount
			symbolResponse:  "0x000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000034441490000000000000000000000000000000000000000000000000000000000",
			wantError:       false,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockExecutionClient{
				balanceOfResponse: tt.balanceResponse,
				symbolResponse:    tt.symbolResponse,
			}

			log := logrus.New()
			log.SetLevel(logrus.ErrorLevel)

			namespace := "erc20_" + string(rune('a'+i))

			erc20 := NewERC20(
				mockClient,
				log,
				15*time.Second,
				namespace,
				map[string]string{},
				[]*AddressERC20{tt.address},
			)

			err := erc20.getBalance(tt.address)

			if (err != nil) != tt.wantError {
				t.Errorf("getBalance() error = %v, wantError %v", err, tt.wantError)
			}

			// Verify both balanceOf and symbol calls were made
			if len(mockClient.callLog) != 2 {
				t.Errorf("Expected 2 RPC calls (balanceOf + symbol), got %d", len(mockClient.callLog))
			}

			if len(mockClient.callLog) > 0 && mockClient.callLog[0].data[:10] != "0x70a08231" {
				t.Errorf("First call should be balanceOf (0x70a08231)")
			}

			if len(mockClient.callLog) > 1 && mockClient.callLog[1].data[:10] != "0x95d89b41" {
				t.Errorf("Second call should be symbol (0x95d89b41)")
			}
		})
	}
}

func TestERC20_tick(t *testing.T) {
	mockClient := &mockExecutionClient{
		balanceOfResponse: "0x0de0b6b3a7640000",
		symbolResponse:    "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000455544800000000000000000000000000000000000000000000000000000000",
		callLog:           []mockCall{},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressERC20{
		{
			Name:     "Token 1",
			Address:  "0x1111111111111111111111111111111111111111",
			Contract: "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Labels:   map[string]string{},
		},
		{
			Name:     "Token 2",
			Address:  "0x2222222222222222222222222222222222222222",
			Contract: "0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			Labels:   map[string]string{},
		},
	}

	erc20 := NewERC20(
		mockClient,
		log,
		15*time.Second,
		"test_erc20_tick",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	erc20.tick(ctx)

	// Each address requires 2 calls (balanceOf + symbol)
	expectedCalls := len(addresses) * 2
	if len(mockClient.callLog) != expectedCalls {
		t.Errorf("Expected %d RPC calls, got %d", expectedCalls, len(mockClient.callLog))
	}
}

func TestERC20_getLabelValues(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressERC20{
		{
			Name:     "Test Token",
			Address:  "0x1234567890123456789012345678901234567890",
			Contract: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
			Labels: map[string]string{
				"type": "stablecoin",
			},
		},
	}

	erc20 := NewERC20(
		&mockExecutionClient{},
		log,
		15*time.Second,
		"test_erc20_labels",
		map[string]string{},
		addresses,
	)

	labels := erc20.getLabelValues(addresses[0], "USDC")

	if len(labels) != len(erc20.labelsMap) {
		t.Errorf("Expected %d label values, got %d", len(erc20.labelsMap), len(labels))
	}

	hasSymbol := false

	for _, label := range labels {
		if label == "USDC" {
			hasSymbol = true
		}
	}

	if !hasSymbol {
		t.Error("Label values should contain the symbol")
	}
}

func TestERC20_Name(t *testing.T) {
	erc20 := &ERC20{}
	if erc20.Name() != NameERC20 {
		t.Errorf("Expected name %s, got %s", NameERC20, erc20.Name())
	}
}
