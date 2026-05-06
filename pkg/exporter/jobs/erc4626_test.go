package jobs

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

//nolint:gocognit,funlen // table-driven test with detailed call verification
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
				Address:  testLidoHolderAddress,
				Contract: testERC4626Vault,
				Labels:   map[string]string{testLabelKeyType: "usdc"},
			},
			balanceOfResponse:       "0x0000000000000000000000000000000000000000000000000de0b6b3a7640000", // 1e18
			convertToAssetsResponse: "0x00000000000000000000000000000000000000000000000000038d7ea4c68000", // 1e15
			wantError:               false,
		},
		{
			name: testNameZeroBal,
			address: &AddressERC4626{
				Name:     "Test Vault Zero",
				Address:  "0x0000000000000000000000000000000000000001",
				Contract: testERC4626Vault,
				Labels:   map[string]string{},
			},
			balanceOfResponse:       "0x0000000000000000000000000000000000000000000000000000000000000000",
			convertToAssetsResponse: "0x0000000000000000000000000000000000000000000000000000000000000000",
			wantError:               false,
		},
		{
			name: testNameLargeBal,
			address: &AddressERC4626{
				Name:     "Test Vault Large",
				Address:  "0x0000000000000000000000000000000000000002",
				Contract: testERC4626Vault,
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
				symbolResponse:          testABISymbolSUSDCResponse, // "sUSDC-Vault"
				balanceOfError:          tt.balanceOfError,
				convertToAssetsError:    tt.convertToAssetsError,
				callLog:                 []mockCall{},
			}

			log := logrus.New()
			log.SetLevel(logrus.ErrorLevel)

			// Use unique namespace for each test to avoid Prometheus registration conflicts
			namespace := "erc4626_" + strconv.Itoa(i)

			erc4626 := NewERC4626(
				mockClients(mockClient),
				log,
				15*time.Second,
				namespace,
				map[string]string{},
				[]*AddressERC4626{tt.address},
			)

			err := erc4626.getAssets(context.Background(), mockClient, tt.address)

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
		symbolResponse:          testABISymbolSUSDCResponse, // "sUSDC-Vault"
		callLog:                 []mockCall{},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressERC4626{
		{
			Name:     "Vault 1",
			Address:  testHolder1Address,
			Contract: testERC4626Vault,
			Labels:   map[string]string{},
		},
		{
			Name:     "Vault 2",
			Address:  testHolder2Address,
			Contract: testERC4626Vault,
			Labels:   map[string]string{},
		},
	}

	erc4626 := NewERC4626(
		mockClients(mockClient),
		log,
		15*time.Second,
		"test_tick",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	erc4626.tick(ctx)

	// Verify that tick called getAssets for each address (1 client * 2 addresses * 3 calls each = 6 total)
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
			Address:  testLidoHolderAddress,
			Contract: testERC4626Vault,
			Labels: map[string]string{
				testLabelKeyType: "usdc",
				"extra":          "custom",
			},
		},
	}

	erc4626 := NewERC4626(
		mockClients(&mockExecutionClient{
			symbolResponse: testABISymbolSUSDCResponse, // "sUSDC-Vault"
		}),
		log,
		15*time.Second,
		"test_labels",
		map[string]string{},
		addresses,
	)

	labels := erc4626.getLabelValues(addresses[0], "sUSDC-Vault", "mock-node")

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
