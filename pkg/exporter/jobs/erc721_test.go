package jobs

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestERC721_getBalance(t *testing.T) {
	tests := []struct {
		name            string
		address         *AddressERC721
		balanceResponse string
		wantError       bool
	}{
		{
			name: "successful NFT balance retrieval",
			address: &AddressERC721{
				Name:     "NFT Collection",
				Address:  testLidoHolderAddress,
				Contract: testBoredApeContract,
				Labels:   map[string]string{testLabelKeyType: "bayc"},
			},
			balanceResponse: "0x0000000000000000000000000000000000000000000000000000000000000005", // 5 NFTs
			wantError:       false,
		},
		{
			name: "zero NFT balance",
			address: &AddressERC721{
				Name:     testNameEmptyWallet,
				Address:  "0x0000000000000000000000000000000000000001",
				Contract: testBoredApeContract,
				Labels:   map[string]string{},
			},
			balanceResponse: "0x0000000000000000000000000000000000000000000000000000000000000000",
			wantError:       false,
		},
		{
			name: "single NFT",
			address: &AddressERC721{
				Name:     "Single NFT Holder",
				Address:  "0x0000000000000000000000000000000000000002",
				Contract: "0x60E4d786628Fea6478F785A6d7e704777c86a7c6",
				Labels:   map[string]string{},
			},
			balanceResponse: "0x0000000000000000000000000000000000000000000000000000000000000001",
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

			namespace := "erc721_" + strconv.Itoa(i)

			erc721 := NewERC721(
				mockClients(mockClient),
				log,
				15*time.Second,
				namespace,
				map[string]string{},
				[]*AddressERC721{tt.address},
			)

			err := erc721.getBalance(context.Background(), mockClient, tt.address)

			if (err != nil) != tt.wantError {
				t.Errorf("getBalance() error = %v, wantError %v", err, tt.wantError)
			}

			// Verify balanceOf call was made
			if len(mockClient.callLog) != 1 {
				t.Errorf("Expected 1 RPC call (balanceOf), got %d", len(mockClient.callLog))
			}

			if len(mockClient.callLog) > 0 && mockClient.callLog[0].data[:10] != "0x70a08231" {
				t.Errorf("Call should be balanceOf (0x70a08231)")
			}
		})
	}
}

func TestERC721_tick(t *testing.T) {
	mockClient := &mockExecutionClient{
		balanceOfResponse: "0x0000000000000000000000000000000000000000000000000000000000000003",
		callLog:           []mockCall{},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressERC721{
		{
			Name:     "NFT 1",
			Address:  testHolder1Address,
			Contract: testContractAAddress,
			Labels:   map[string]string{},
		},
		{
			Name:     "NFT 2",
			Address:  testHolder2Address,
			Contract: testContractBAddress,
			Labels:   map[string]string{},
		},
	}

	erc721 := NewERC721(
		mockClients(mockClient),
		log,
		15*time.Second,
		"test_erc721_tick",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	erc721.tick(ctx)

	// Each address requires 1 call (balanceOf), 1 client * 2 addresses
	expectedCalls := len(addresses)
	if len(mockClient.callLog) != expectedCalls {
		t.Errorf("Expected %d RPC calls, got %d", expectedCalls, len(mockClient.callLog))
	}
}

func TestERC721_getLabelValues(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressERC721{
		{
			Name:     "Test NFT",
			Address:  testLidoHolderAddress,
			Contract: testBoredApeContract,
			Labels: map[string]string{
				testLabelKeyType: "bayc",
			},
		},
	}

	erc721 := NewERC721(
		mockClients(&mockExecutionClient{}),
		log,
		15*time.Second,
		"test_erc721_labels",
		map[string]string{},
		addresses,
	)

	labels := erc721.getLabelValues(addresses[0], "mock-node")

	if len(labels) != len(erc721.labelsMap) {
		t.Errorf("Expected %d label values, got %d", len(erc721.labelsMap), len(labels))
	}

	hasName := false
	hasContract := false

	for _, label := range labels {
		if label == addresses[0].Name {
			hasName = true
		}

		if label == addresses[0].Contract {
			hasContract = true
		}
	}

	if !hasName {
		t.Error("Label values should contain the name")
	}

	if !hasContract {
		t.Error("Label values should contain the contract")
	}
}

func TestERC721_Name(t *testing.T) {
	erc721 := &ERC721{}
	if erc721.Name() != NameERC721 {
		t.Errorf("Expected name %s, got %s", NameERC721, erc721.Name())
	}
}
