package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
	"github.com/sirupsen/logrus"
)

// mockExecutionClient is a mock implementation of the ExecutionClient interface for testing.
type mockExecutionClient struct {
	balanceOfResponse       string
	convertToAssetsResponse string
	symbolResponse          string
	getReservesResponse     string
	latestAnswerResponse    string
	balanceOfError          error
	convertToAssetsError    error
	symbolError             error
	callLog                 []mockCall
	ethGetBalanceResponse   string
	ethGetBalanceError      error
	ethGetBalanceCalls      int
}

type mockCall struct {
	to   string
	data string
}

func (m *mockExecutionClient) ETHCall(transaction *api.ETHCallTransaction, block string) (string, error) {
	m.callLog = append(m.callLog, mockCall{
		to:   transaction.To,
		data: *transaction.Data,
	})

	// Check if this is a balanceOf call (0x70a08231)
	if len(*transaction.Data) >= 10 && (*transaction.Data)[:10] == "0x70a08231" {
		if m.balanceOfError != nil {
			return "", m.balanceOfError
		}

		return m.balanceOfResponse, nil
	}

	// Check if this is a convertToAssets call (0x07a2d13a)
	if len(*transaction.Data) >= 10 && (*transaction.Data)[:10] == "0x07a2d13a" {
		if m.convertToAssetsError != nil {
			return "", m.convertToAssetsError
		}

		return m.convertToAssetsResponse, nil
	}

	// Check if this is a symbol call (0x95d89b41)
	if len(*transaction.Data) >= 10 && (*transaction.Data)[:10] == "0x95d89b41" {
		if m.symbolError != nil {
			return "", m.symbolError
		}

		if m.symbolResponse != "" {
			return m.symbolResponse, nil
		}

		return "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000455544800000000000000000000000000000000000000000000000000000000", nil
	}

	// Check if this is a getReserves call (0x0902f1ac)
	if len(*transaction.Data) >= 10 && (*transaction.Data)[:10] == "0x0902f1ac" {
		if m.getReservesResponse != "" {
			return m.getReservesResponse, nil
		}

		return "0x0000000000000000000000000000000000000000000000000de0b6b3a76400000000000000000000000000000000000000000000000000000de0b6b3a7640000", nil
	}

	// Check if this is a latestAnswer call (0x50d25bcd)
	if len(*transaction.Data) >= 10 && (*transaction.Data)[:10] == "0x50d25bcd" {
		if m.latestAnswerResponse != "" {
			return m.latestAnswerResponse, nil
		}

		return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
	}

	// Check if this is a balanceOf for ERC1155 (0x00fdd58e)
	if len(*transaction.Data) >= 10 && (*transaction.Data)[:10] == "0x00fdd58e" {
		if m.balanceOfError != nil {
			return "", m.balanceOfError
		}

		return m.balanceOfResponse, nil
	}

	return "0x0", nil
}

func (m *mockExecutionClient) ETHGetBalance(address string, block string) (string, error) {
	m.ethGetBalanceCalls++

	if m.ethGetBalanceError != nil {
		return "", m.ethGetBalanceError
	}

	if m.ethGetBalanceResponse != "" {
		return m.ethGetBalanceResponse, nil
	}

	return "0x0", nil
}

func TestERC4626_getAssets(t *testing.T) {
	tests := []struct {
		name                    string
		address                 *AddressERC4626
		balanceOfResponse       string
		convertToAssetsResponse string
		balanceOfError          error
		convertToAssetsError    error
		wantError               bool
	}{
		{
			name: "successful conversion with shares balance",
			address: &AddressERC4626{
				Name:     "Test Vault",
				Address:  "0x1234567890123456789012345678901234567890",
				Contract: "0xBEEF01735c132Ada46AA9aA4c54623cAA92A64CB",
				Labels:   map[string]string{"type": "usdc"},
			},
			balanceOfResponse:       "0x0000000000000000000000000000000000000000000000000de0b6b3a7640000", // 1e18
			convertToAssetsResponse: "0x00000000000000000000000000000000000000000000000000038d7ea4c68000", // 1e15
			wantError:               false,
		},
		{
			name: "zero balance",
			address: &AddressERC4626{
				Name:     "Test Vault Zero",
				Address:  "0x0000000000000000000000000000000000000001",
				Contract: "0xBEEF01735c132Ada46AA9aA4c54623cAA92A64CB",
				Labels:   map[string]string{},
			},
			balanceOfResponse:       "0x0000000000000000000000000000000000000000000000000000000000000000",
			convertToAssetsResponse: "0x0000000000000000000000000000000000000000000000000000000000000000",
			wantError:               false,
		},
		{
			name: "large balance",
			address: &AddressERC4626{
				Name:     "Test Vault Large",
				Address:  "0x0000000000000000000000000000000000000002",
				Contract: "0xBEEF01735c132Ada46AA9aA4c54623cAA92A64CB",
				Labels:   map[string]string{},
			},
			balanceOfResponse:       "0x0000000000000000000000000000000000000000000000056bc75e2d63100000",   // 1e20
			convertToAssetsResponse: "0x0000000000000000000000000000000000000000000000152d02c7e14af6800000", // large number
			wantError:               false,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockExecutionClient{
				balanceOfResponse:       tt.balanceOfResponse,
				convertToAssetsResponse: tt.convertToAssetsResponse,
				symbolResponse:          "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000973555344432d5661756c74000000000000000000000000000000000000000000", // "sUSDC-Vault"
				balanceOfError:          tt.balanceOfError,
				convertToAssetsError:    tt.convertToAssetsError,
				callLog:                 []mockCall{},
			}

			log := logrus.New()
			log.SetLevel(logrus.ErrorLevel)

			// Use unique namespace for each test to avoid Prometheus registration conflicts
			namespace := string(rune('a' + i))

			erc4626 := NewERC4626(
				mockClient,
				log,
				15*time.Second,
				namespace,
				map[string]string{},
				[]*AddressERC4626{tt.address},
			)

			err := erc4626.getAssets(tt.address)

			if (err != nil) != tt.wantError {
				t.Errorf("getAssets() error = %v, wantError %v", err, tt.wantError)
			}

			// Verify that all three calls were made in the correct order
			if len(mockClient.callLog) != 3 {
				t.Errorf("Expected 3 RPC calls (balanceOf + convertToAssets + symbol), got %d", len(mockClient.callLog))
			}

			// Verify first call was balanceOf
			if len(mockClient.callLog) > 0 {
				firstCall := mockClient.callLog[0]
				if firstCall.to != tt.address.Contract {
					t.Errorf("First call should be to contract %s, got %s", tt.address.Contract, firstCall.to)
				}

				if len(firstCall.data) < 10 || firstCall.data[:10] != "0x70a08231" {
					t.Errorf("First call should be balanceOf (0x70a08231), got %s", firstCall.data[:10])
				}
			}

			// Verify second call was convertToAssets
			if len(mockClient.callLog) > 1 {
				secondCall := mockClient.callLog[1]
				if secondCall.to != tt.address.Contract {
					t.Errorf("Second call should be to contract %s, got %s", tt.address.Contract, secondCall.to)
				}

				if len(secondCall.data) < 10 || secondCall.data[:10] != "0x07a2d13a" {
					t.Errorf("Second call should be convertToAssets (0x07a2d13a), got %s", secondCall.data[:10])
				}
			}

			// Verify third call was symbol
			if len(mockClient.callLog) > 2 {
				thirdCall := mockClient.callLog[2]
				if thirdCall.to != tt.address.Contract {
					t.Errorf("Third call should be to contract %s, got %s", tt.address.Contract, thirdCall.to)
				}

				if len(thirdCall.data) < 10 || thirdCall.data[:10] != "0x95d89b41" {
					t.Errorf("Third call should be symbol (0x95d89b41), got %s", thirdCall.data[:10])
				}
			}
		})
	}
}

func TestERC4626_tick(t *testing.T) {
	mockClient := &mockExecutionClient{
		balanceOfResponse:       "0x0000000000000000000000000000000000000000000000000de0b6b3a7640000",
		convertToAssetsResponse: "0x00000000000000000000000000000000000000000000000000038d7ea4c68000",
		symbolResponse:          "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000973555344432d5661756c74000000000000000000000000000000000000000000", // "sUSDC-Vault"
		callLog:                 []mockCall{},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressERC4626{
		{
			Name:     "Vault 1",
			Address:  "0x1111111111111111111111111111111111111111",
			Contract: "0xBEEF01735c132Ada46AA9aA4c54623cAA92A64CB",
			Labels:   map[string]string{},
		},
		{
			Name:     "Vault 2",
			Address:  "0x2222222222222222222222222222222222222222",
			Contract: "0xBEEF01735c132Ada46AA9aA4c54623cAA92A64CB",
			Labels:   map[string]string{},
		},
	}

	erc4626 := NewERC4626(
		mockClient,
		log,
		15*time.Second,
		"test_tick",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	erc4626.tick(ctx)

	// Verify that tick called getAssets for each address (2 addresses * 3 calls each = 6 total)
	expectedCalls := len(addresses) * 3
	if len(mockClient.callLog) != expectedCalls {
		t.Errorf("Expected %d RPC calls for %d addresses, got %d", expectedCalls, len(addresses), len(mockClient.callLog))
	}
}

func TestERC4626_getLabelValues(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressERC4626{
		{
			Name:     "Test Vault",
			Address:  "0x1234567890123456789012345678901234567890",
			Contract: "0xBEEF01735c132Ada46AA9aA4c54623cAA92A64CB",
			Labels: map[string]string{
				"type":  "usdc",
				"extra": "custom",
			},
		},
	}

	erc4626 := NewERC4626(
		&mockExecutionClient{
			symbolResponse: "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000973555344432d5661756c74000000000000000000000000000000000000000000", // "sUSDC-Vault"
		},
		log,
		15*time.Second,
		"test_labels",
		map[string]string{},
		addresses,
	)

	labels := erc4626.getLabelValues(addresses[0], "sUSDC-Vault")

	// Verify label values are populated correctly
	if len(labels) != len(erc4626.labelsMap) {
		t.Errorf("Expected %d label values, got %d", len(erc4626.labelsMap), len(labels))
	}

	// Verify standard labels are present
	hasName := false
	hasAddress := false
	hasContract := false
	hasSymbol := false

	for _, label := range labels {
		if label == addresses[0].Name {
			hasName = true
		}

		if label == addresses[0].Address {
			hasAddress = true
		}

		if label == addresses[0].Contract {
			hasContract = true
		}

		if label == "sUSDC-Vault" {
			hasSymbol = true
		}
	}

	if !hasName {
		t.Error("Label values should contain the name")
	}

	if !hasAddress {
		t.Error("Label values should contain the address")
	}

	if !hasContract {
		t.Error("Label values should contain the contract")
	}

	if !hasSymbol {
		t.Error("Label values should contain the symbol")
	}
}

func TestERC4626_Name(t *testing.T) {
	erc4626 := &ERC4626{}
	if erc4626.Name() != NameERC4626 {
		t.Errorf("Expected name %s, got %s", NameERC4626, erc4626.Name())
	}
}
