package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
)

func TestAccount_MultiClient_QueriesAllNodes(t *testing.T) {
	t.Parallel()

	client1 := &mockExecutionClient{
		name:                  "geth-1",
		ethGetBalanceResponse: "0xde0b6b3a7640000", // 1 ETH
	}

	client2 := &mockExecutionClient{
		name:                  "nethermind-1",
		ethGetBalanceResponse: "0xde0b6b3a7640000",
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	clients := []api.ExecutionClient{client1, client2}

	addresses := []*AddressAccount{
		{
			Name:    testNameTestAccount,
			Address: testHolder1Address,
			Labels:  map[string]string{},
		},
	}

	account := NewAccount(
		clients,
		log,
		15*time.Second,
		"multi_client_account",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	account.tick(ctx)

	// Both clients should have been called
	assert.Equal(t, 1, client1.ethGetBalanceCalls, "client1 should be called once")
	assert.Equal(t, 1, client2.ethGetBalanceCalls, "client2 should be called once")

	// Verify metrics have the execution label for each client
	gethVal := testutil.ToFloat64(account.AccountBalance.WithLabelValues(testNameTestAccount, testHolder1Address, "geth-1"))
	nethermindVal := testutil.ToFloat64(account.AccountBalance.WithLabelValues(testNameTestAccount, testHolder1Address, "nethermind-1"))

	assert.Greater(t, gethVal, 0.0, "geth-1 metric should be set")
	assert.Greater(t, nethermindVal, 0.0, "nethermind-1 metric should be set")
	assert.InDelta(t, gethVal, nethermindVal, 1.0, "both nodes should report same balance")
}

func TestAccount_MultiClient_OneNodeFails(t *testing.T) {
	t.Parallel()

	client1 := &mockExecutionClient{
		name:                  "geth-1",
		ethGetBalanceResponse: "0xde0b6b3a7640000",
	}

	client2 := &mockExecutionClient{
		name:               "broken-node",
		ethGetBalanceError: errors.New("connection refused"),
	}

	client3 := &mockExecutionClient{
		name:                  "nethermind-1",
		ethGetBalanceResponse: "0xde0b6b3a7640000",
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	clients := []api.ExecutionClient{client1, client2, client3}

	addresses := []*AddressAccount{
		{
			Name:    testNameTestAccount,
			Address: testHolder1Address,
			Labels:  map[string]string{},
		},
	}

	account := NewAccount(
		clients,
		log,
		15*time.Second,
		"multi_client_error",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	account.tick(ctx)

	// All clients should have been attempted
	assert.Equal(t, 1, client1.ethGetBalanceCalls, "client1 should be called")
	assert.Equal(t, 1, client2.ethGetBalanceCalls, "broken client should be called")
	assert.Equal(t, 1, client3.ethGetBalanceCalls, "client3 should still be called after client2 fails")

	// Working nodes should have metrics
	gethVal := testutil.ToFloat64(account.AccountBalance.WithLabelValues(testNameTestAccount, testHolder1Address, "geth-1"))
	assert.Greater(t, gethVal, 0.0)

	nethermindVal := testutil.ToFloat64(account.AccountBalance.WithLabelValues(testNameTestAccount, testHolder1Address, "nethermind-1"))
	assert.Greater(t, nethermindVal, 0.0)

	// Broken node should have error counter incremented
	errVal := testutil.ToFloat64(account.AccountError.WithLabelValues(testNameTestAccount, testHolder1Address, "broken-node"))
	assert.InDelta(t, 1.0, errVal, 0.01, "error counter should be incremented for broken node")
}

func TestAccount_MultiClient_DifferentBalances(t *testing.T) {
	t.Parallel()

	client1 := &mockExecutionClient{
		name:                  "node-a",
		ethGetBalanceResponse: "0xde0b6b3a7640000", // 1 ETH
	}

	client2 := &mockExecutionClient{
		name:                  "node-b",
		ethGetBalanceResponse: "0x1bc16d674ec80000", // 2 ETH
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	clients := []api.ExecutionClient{client1, client2}

	addresses := []*AddressAccount{
		{
			Name:    "Divergent",
			Address: testHolder1Address,
			Labels:  map[string]string{},
		},
	}

	account := NewAccount(
		clients,
		log,
		15*time.Second,
		"multi_client_diverge",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	account.tick(ctx)

	nodeAVal := testutil.ToFloat64(account.AccountBalance.WithLabelValues("Divergent", testHolder1Address, "node-a"))
	nodeBVal := testutil.ToFloat64(account.AccountBalance.WithLabelValues("Divergent", testHolder1Address, "node-b"))

	// Different nodes report different balances — this is the whole point of multi-node
	assert.NotEqual(t, nodeAVal, nodeBVal, "nodes should report different balances for consensus detection")
	assert.InDelta(t, 1e18, nodeAVal, 1.0)
	assert.InDelta(t, 2e18, nodeBVal, 1.0)
}

func TestERC20_MultiClient_QueriesAllNodes(t *testing.T) {
	t.Parallel()

	client1 := &mockExecutionClient{
		name:              testNodeName1,
		balanceOfResponse: "0x0000000000000000000000000000000000000000000000000000000005f5e100",
	}

	client2 := &mockExecutionClient{
		name:              "node-2",
		balanceOfResponse: "0x0000000000000000000000000000000000000000000000000000000005f5e100",
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	clients := []api.ExecutionClient{client1, client2}

	addresses := []*AddressERC20{
		{
			Name:     testNameUSDC,
			Address:  testHolder1Address,
			Contract: testUSDCContract,
			Labels:   map[string]string{},
		},
	}

	erc20 := NewERC20(
		clients,
		log,
		15*time.Second,
		"multi_client_erc20",
		map[string]string{},
		addresses,
	)

	ctx := context.Background()
	erc20.tick(ctx)

	// Each client should make 2 calls per address (balanceOf + symbol)
	assert.Len(t, client1.callLog, 2, "node-1 should make 2 calls")
	assert.Len(t, client2.callLog, 2, "node-2 should make 2 calls")
}

func TestAccount_Start_ContextCancellation(t *testing.T) {
	t.Parallel()

	mockClient := &mockExecutionClient{
		name:                  testNodeName1,
		ethGetBalanceResponse: "0x0",
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	account := NewAccount(
		mockClients(mockClient),
		log,
		50*time.Millisecond, // short interval for testing
		"start_cancel",
		map[string]string{},
		[]*AddressAccount{
			{Name: "test", Address: testHolder1Address, Labels: map[string]string{}},
		},
	)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		account.Start(ctx)
		close(done)
	}()

	// Let at least one tick happen
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Start should return after context cancellation
	select {
	case <-done:
		// Success - Start returned
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}

	require.GreaterOrEqual(t, mockClient.ethGetBalanceCalls, 1, "at least one tick should have run")
}

func TestAccount_ExecutionLabelInMetrics(t *testing.T) {
	t.Parallel()

	mockClient := &mockExecutionClient{
		name:                  "my-custom-node",
		ethGetBalanceResponse: "0xde0b6b3a7640000",
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	account := NewAccount(
		mockClients(mockClient),
		log,
		15*time.Second,
		"exec_label_test",
		map[string]string{},
		[]*AddressAccount{
			{Name: "addr1", Address: testHolder1Address, Labels: map[string]string{}},
		},
	)

	ctx := context.Background()
	account.tick(ctx)

	// Verify the execution label is set to the client name
	val := testutil.ToFloat64(account.AccountBalance.WithLabelValues("addr1", testHolder1Address, "my-custom-node"))
	assert.InDelta(t, 1e18, val, 1.0)

	// Verify the metric count - should be exactly 1 series
	count := testutil.CollectAndCount(&account.AccountBalance)
	assert.Equal(t, 1, count)
}
