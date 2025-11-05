package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestAccount_getBalance(t *testing.T) {
	tests := []struct {
		name            string
		address         *AddressAccount
		balanceResponse string
		balanceError    error
		wantError       bool
	}{
		{
			name: "successful balance retrieval",
			address: &AddressAccount{
				Name:    "Test Account",
				Address: "0x1234567890123456789012345678901234567890",
				Labels:  map[string]string{"type": "friend"},
			},
			balanceResponse: "0x0de0b6b3a7640000", // 1 ETH
			wantError:       false,
		},
		{
			name: "zero balance",
			address: &AddressAccount{
				Name:    "Empty Account",
				Address: "0x0000000000000000000000000000000000000001",
				Labels:  map[string]string{},
			},
			balanceResponse: "0x0",
			wantError:       false,
		},
		{
			name: "large balance",
			address: &AddressAccount{
				Name:    "Whale Account",
				Address: "0x0000000000000000000000000000000000000002",
				Labels:  map[string]string{},
			},
			balanceResponse: "0x56bc75e2d63100000", // 100 ETH
			wantError:       false,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockExecutionClient{
				balanceOfResponse: tt.balanceResponse,
				balanceOfError:    tt.balanceError,
			}

			// Override ETHGetBalance for account tests
			mockClient.ethGetBalanceResponse = tt.balanceResponse
			mockClient.ethGetBalanceError = tt.balanceError

			log := logrus.New()
			log.SetLevel(logrus.ErrorLevel)

			namespace := string(rune('a' + i))

			account := NewAccount(
				mockClient,
				log,
				15*time.Second,
				namespace,
				map[string]string{},
				[]*AddressAccount{tt.address},
			)

			err := account.getBalance(tt.address)

			if (err != nil) != tt.wantError {
				t.Errorf("getBalance() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestAccount_tick(t *testing.T) {
	mockClient := &mockExecutionClient{
		ethGetBalanceResponse: "0x0de0b6b3a7640000",
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressAccount{
		{
			Name:    "Account 1",
			Address: "0x1111111111111111111111111111111111111111",
			Labels:  map[string]string{},
		},
		{
			Name:    "Account 2",
			Address: "0x2222222222222222222222222222222222222222",
			Labels:  map[string]string{},
		},
	}

	account := NewAccount(
		mockClient,
		log,
		15*time.Second,
		"test_account_tick",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	account.tick(ctx)

	// ETHGetBalance calls are tracked differently than ETHCall
	// The mock should have been called for each address
	if mockClient.ethGetBalanceCalls != len(addresses) {
		t.Errorf("Expected %d ETHGetBalance calls, got %d", len(addresses), mockClient.ethGetBalanceCalls)
	}
}

func TestAccount_getLabelValues(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressAccount{
		{
			Name:    "Test Account",
			Address: "0x1234567890123456789012345678901234567890",
			Labels: map[string]string{
				"type":  "friend",
				"extra": "custom",
			},
		},
	}

	account := NewAccount(
		&mockExecutionClient{},
		log,
		15*time.Second,
		"test_account_labels",
		map[string]string{},
		addresses,
	)

	labels := account.getLabelValues(addresses[0])

	if len(labels) != len(account.labelsMap) {
		t.Errorf("Expected %d label values, got %d", len(account.labelsMap), len(labels))
	}

	hasName := false
	hasAddress := false

	for _, label := range labels {
		if label == addresses[0].Name {
			hasName = true
		}

		if label == addresses[0].Address {
			hasAddress = true
		}
	}

	if !hasName {
		t.Error("Label values should contain the name")
	}

	if !hasAddress {
		t.Error("Label values should contain the address")
	}
}

func TestAccount_Name(t *testing.T) {
	account := &Account{}
	if account.Name() != NameAccount {
		t.Errorf("Expected name %s, got %s", NameAccount, account.Name())
	}
}
