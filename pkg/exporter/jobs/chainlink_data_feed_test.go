package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestChainlinkDataFeed_getBalance(t *testing.T) {
	tests := []struct {
		name                 string
		address              *AddressChainlinkDataFeed
		latestAnswerResponse string
		wantError            bool
	}{
		{
			name: "successful price retrieval",
			address: &AddressChainlinkDataFeed{
				Name:     "ETH/USD",
				From:     "eth",
				To:       "usd",
				Contract: "0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419",
				Labels:   map[string]string{"type": "chainlink"},
			},
			latestAnswerResponse: "0x00000000000000000000000000000000000000000000000000000000773594c0", // ~2000 USD
			wantError:            false,
		},
		{
			name: "zero price",
			address: &AddressChainlinkDataFeed{
				Name:     "TEST/USD",
				From:     "test",
				To:       "usd",
				Contract: "0x0000000000000000000000000000000000000001",
				Labels:   map[string]string{},
			},
			latestAnswerResponse: "0x0000000000000000000000000000000000000000000000000000000000000000",
			wantError:            false,
		},
		{
			name: "high price",
			address: &AddressChainlinkDataFeed{
				Name:     "BTC/USD",
				From:     "btc",
				To:       "usd",
				Contract: "0xF4030086522a5bEEa4988F8cA5B36dbC97BeE88c",
				Labels:   map[string]string{},
			},
			latestAnswerResponse: "0x0000000000000000000000000000000000000000000000000000000ba43b7400", // ~50000 USD
			wantError:            false,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockExecutionClient{
				latestAnswerResponse: tt.latestAnswerResponse,
			}

			log := logrus.New()
			log.SetLevel(logrus.ErrorLevel)

			namespace := "chainlink_" + string(rune('a'+i))

			chainlink := NewChainlinkDataFeed(
				mockClient,
				log,
				15*time.Second,
				namespace,
				map[string]string{},
				[]*AddressChainlinkDataFeed{tt.address},
			)

			err := chainlink.getBalance(tt.address)

			if (err != nil) != tt.wantError {
				t.Errorf("getBalance() error = %v, wantError %v", err, tt.wantError)
			}

			// Verify latestAnswer call was made
			if len(mockClient.callLog) != 1 {
				t.Errorf("Expected 1 RPC call (latestAnswer), got %d", len(mockClient.callLog))
			}

			if len(mockClient.callLog) > 0 && mockClient.callLog[0].data[:10] != "0x50d25bcd" {
				t.Errorf("Call should be latestAnswer (0x50d25bcd), got %s", mockClient.callLog[0].data[:10])
			}
		})
	}
}

func TestChainlinkDataFeed_tick(t *testing.T) {
	mockClient := &mockExecutionClient{
		latestAnswerResponse: "0x00000000000000000000000000000000000000000000000000000000773594c0",
		callLog:              []mockCall{},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressChainlinkDataFeed{
		{
			Name:     "ETH/USD",
			From:     "eth",
			To:       "usd",
			Contract: "0x1111111111111111111111111111111111111111",
			Labels:   map[string]string{},
		},
		{
			Name:     "BTC/USD",
			From:     "btc",
			To:       "usd",
			Contract: "0x2222222222222222222222222222222222222222",
			Labels:   map[string]string{},
		},
	}

	chainlink := NewChainlinkDataFeed(
		mockClient,
		log,
		15*time.Second,
		"test_chainlink_tick",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	chainlink.tick(ctx)

	// Each address requires 1 call (latestAnswer)
	expectedCalls := len(addresses)
	if len(mockClient.callLog) != expectedCalls {
		t.Errorf("Expected %d RPC calls, got %d", expectedCalls, len(mockClient.callLog))
	}
}

func TestChainlinkDataFeed_getLabelValues(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressChainlinkDataFeed{
		{
			Name:     "ETH/USD",
			From:     "eth",
			To:       "usd",
			Contract: "0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419",
			Labels: map[string]string{
				"type": "chainlink",
			},
		},
	}

	chainlink := NewChainlinkDataFeed(
		&mockExecutionClient{},
		log,
		15*time.Second,
		"test_chainlink_labels",
		map[string]string{},
		addresses,
	)

	labels := chainlink.getLabelValues(addresses[0])

	if len(labels) != len(chainlink.labelsMap) {
		t.Errorf("Expected %d label values, got %d", len(chainlink.labelsMap), len(labels))
	}

	hasFrom := false
	hasTo := false

	for _, label := range labels {
		if label == addresses[0].From {
			hasFrom = true
		}

		if label == addresses[0].To {
			hasTo = true
		}
	}

	if !hasFrom {
		t.Error("Label values should contain the from symbol")
	}

	if !hasTo {
		t.Error("Label values should contain the to symbol")
	}
}

func TestChainlinkDataFeed_Name(t *testing.T) {
	chainlink := &ChainlinkDataFeed{}
	if chainlink.Name() != NameChainlinkDataFeed {
		t.Errorf("Expected name %s, got %s", NameChainlinkDataFeed, chainlink.Name())
	}
}
