package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestUniswapPair_getBalance(t *testing.T) {
	tests := []struct {
		name                string
		address             *AddressUniswapPair
		getReservesResponse string
		wantError           bool
	}{
		{
			name: "successful reserves retrieval",
			address: &AddressUniswapPair{
				Name:     "ETH/USDT",
				From:     "eth",
				To:       "usdt",
				Contract: "0x0d4a11d5eeaac28ec3f61d100daf4d40471f1852",
				Labels:   map[string]string{"type": "uniswap"},
			},
			getReservesResponse: "0x0000000000000000000000000000000000000000000000000de0b6b3a76400000000000000000000000000000000000000000000000000000de0b6b3a7640000", // Equal reserves
			wantError:           false,
		},
		{
			name: "unequal reserves",
			address: &AddressUniswapPair{
				Name:     "DAI/USDC",
				From:     "dai",
				To:       "usdc",
				Contract: "0xAE461cA67B15dc8dc81CE7615e0320dA1A9aB8D5",
				Labels:   map[string]string{},
			},
			getReservesResponse: "0x00000000000000000000000000000000000000000000000001bc16d674ec800000000000000000000000000000000000000000000000000000de0b6b3a7640000", // Different reserves
			wantError:           false,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockExecutionClient{
				getReservesResponse: tt.getReservesResponse,
			}

			log := logrus.New()
			log.SetLevel(logrus.ErrorLevel)

			namespace := "uniswap_" + string(rune('a'+i))

			uniswap := NewUniswapPair(
				mockClient,
				log,
				15*time.Second,
				namespace,
				map[string]string{},
				[]*AddressUniswapPair{tt.address},
			)

			err := uniswap.getBalance(tt.address)

			if (err != nil) != tt.wantError {
				t.Errorf("getBalance() error = %v, wantError %v", err, tt.wantError)
			}

			// Verify getReserves call was made
			if len(mockClient.callLog) != 1 {
				t.Errorf("Expected 1 RPC call (getReserves), got %d", len(mockClient.callLog))
			}

			if len(mockClient.callLog) > 0 && mockClient.callLog[0].data[:10] != "0x0902f1ac" {
				t.Errorf("Call should be getReserves (0x0902f1ac), got %s", mockClient.callLog[0].data[:10])
			}
		})
	}
}

func TestUniswapPair_tick(t *testing.T) {
	mockClient := &mockExecutionClient{
		getReservesResponse: "0x0000000000000000000000000000000000000000000000000de0b6b3a76400000000000000000000000000000000000000000000000000000de0b6b3a7640000",
		callLog:             []mockCall{},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressUniswapPair{
		{
			Name:     "Pair 1",
			From:     "eth",
			To:       "usdt",
			Contract: "0x1111111111111111111111111111111111111111",
			Labels:   map[string]string{},
		},
		{
			Name:     "Pair 2",
			From:     "dai",
			To:       "usdc",
			Contract: "0x2222222222222222222222222222222222222222",
			Labels:   map[string]string{},
		},
	}

	uniswap := NewUniswapPair(
		mockClient,
		log,
		15*time.Second,
		"test_uniswap_tick",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	uniswap.tick(ctx)

	// Each address requires 1 call (getReserves)
	expectedCalls := len(addresses)
	if len(mockClient.callLog) != expectedCalls {
		t.Errorf("Expected %d RPC calls, got %d", expectedCalls, len(mockClient.callLog))
	}
}

func TestUniswapPair_getLabelValues(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	addresses := []*AddressUniswapPair{
		{
			Name:     "ETH/USDT",
			From:     "eth",
			To:       "usdt",
			Contract: "0x0d4a11d5eeaac28ec3f61d100daf4d40471f1852",
			Labels: map[string]string{
				"type": "uniswap",
			},
		},
	}

	uniswap := NewUniswapPair(
		&mockExecutionClient{},
		log,
		15*time.Second,
		"test_uniswap_labels",
		map[string]string{},
		addresses,
	)

	labels := uniswap.getLabelValues(addresses[0])

	if len(labels) != len(uniswap.labelsMap) {
		t.Errorf("Expected %d label values, got %d", len(uniswap.labelsMap), len(labels))
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

func TestUniswapPair_Name(t *testing.T) {
	uniswap := &UniswapPair{}
	if uniswap.Name() != NameUniswapPair {
		t.Errorf("Expected name %s, got %s", NameUniswapPair, uniswap.Name())
	}
}
