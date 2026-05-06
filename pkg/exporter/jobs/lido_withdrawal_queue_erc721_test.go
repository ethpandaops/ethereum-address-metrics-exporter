package jobs

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
)

const (
	testLidoHolderAddress = "0x1234567890123456789012345678901234567890"
	testLidoQueueContract = "0x889edC2eDab5f40e902b864aD4d7AdE8E412F9B1"
	testMockNodeName      = "mock-node"
	testNodeName1         = "node-1"
	testLabelKeyType      = "type"
)

func TestLidoWithdrawalQueueERC721_getWithdrawalQueue(t *testing.T) {
	address := &AddressLidoWithdrawalQueueERC721{
		Name:     "Test Queue",
		Address:  testLidoHolderAddress,
		Contract: testLidoQueueContract,
		Labels:   map[string]string{},
	}

	mockClient := &mockExecutionClient{
		underlyingTokenResponse:    encodeABIAddressReturn("0xabCDEFabcdefABCDefABcdefabcdeFabcdefABCD"),
		symbolResponse:             encodeABIStringReturn("abcETH"),
		decimalsResponse:           encodeABIUintReturn(6),
		withdrawalRequestsResponse: encodeABIUintArrayReturn(101, 102, 103),
		withdrawalStatusResponse: encodeLidoWithdrawalStatusesReturn(
			testWithdrawalStatus(1500000, false, false),
			testWithdrawalStatus(2500000, true, false),
			testWithdrawalStatus(3000000, true, true),
		),
	}

	queue := NewLidoWithdrawalQueueERC721(
		mockClients(mockClient),
		testLogger(),
		15*time.Second,
		"lido_queue_get",
		map[string]string{},
		[]*AddressLidoWithdrawalQueueERC721{address},
	)

	err := queue.getWithdrawalQueue(context.Background(), mockClient, address)
	if err != nil {
		t.Fatalf("getWithdrawalQueue() error = %v", err)
	}

	labels := []string{
		"Test Queue",
		testLidoHolderAddress,
		testLidoQueueContract,
		"abcETH",
		"mock-node",
	}

	assertMetricValue(t, queue.LidoWithdrawalQueueERC721RequestCount, labels, 3)
	assertMetricValue(t, queue.LidoWithdrawalQueueERC721Pending, labels, 1.5)
	assertMetricValue(t, queue.LidoWithdrawalQueueERC721Claimable, labels, 2.5)
	assertMetricValue(t, queue.LidoWithdrawalQueueERC721Claimed, labels, 3)

	wantSelectors := []string{
		lidoWithdrawalQueueUnderlyingTokenSelector,
		lidoWithdrawalQueueSymbolSelector,
		lidoWithdrawalQueueDecimalsSelector,
		lidoWithdrawalQueueGetWithdrawalRequestsSelector,
		lidoWithdrawalQueueGetWithdrawalStatusSelector,
	}

	assertCallSelectors(t, mockClient.callLog, wantSelectors)
}

func TestLidoWithdrawalQueueERC721_getWithdrawalQueue_EmptyRequests(t *testing.T) {
	address := &AddressLidoWithdrawalQueueERC721{
		Name:     "Empty Queue",
		Address:  testLidoHolderAddress,
		Contract: testLidoQueueContract,
		Labels:   map[string]string{},
	}

	mockClient := &mockExecutionClient{
		underlyingTokenResponse:    encodeABIAddressReturn("0x1111111111111111111111111111111111111111"),
		symbolResponse:             encodeABIStringReturn("defETH"),
		decimalsResponse:           encodeABIUintReturn(18),
		withdrawalRequestsResponse: encodeABIUintArrayReturn(),
	}

	queue := NewLidoWithdrawalQueueERC721(
		mockClients(mockClient),
		testLogger(),
		15*time.Second,
		"lido_queue_empty",
		map[string]string{},
		[]*AddressLidoWithdrawalQueueERC721{address},
	)

	err := queue.getWithdrawalQueue(context.Background(), mockClient, address)
	if err != nil {
		t.Fatalf("getWithdrawalQueue() error = %v", err)
	}

	labels := []string{
		"Empty Queue",
		testLidoHolderAddress,
		testLidoQueueContract,
		"defETH",
		"mock-node",
	}

	assertMetricValue(t, queue.LidoWithdrawalQueueERC721RequestCount, labels, 0)
	assertMetricValue(t, queue.LidoWithdrawalQueueERC721Pending, labels, 0)
	assertMetricValue(t, queue.LidoWithdrawalQueueERC721Claimable, labels, 0)
	assertMetricValue(t, queue.LidoWithdrawalQueueERC721Claimed, labels, 0)
	assertCallSelectors(t, mockClient.callLog, []string{
		lidoWithdrawalQueueUnderlyingTokenSelector,
		lidoWithdrawalQueueSymbolSelector,
		lidoWithdrawalQueueDecimalsSelector,
		lidoWithdrawalQueueGetWithdrawalRequestsSelector,
	})
}

func TestLidoWithdrawalQueueERC721_tick_MultiClient(t *testing.T) {
	address := &AddressLidoWithdrawalQueueERC721{
		Name:     "Multi Queue",
		Address:  testLidoHolderAddress,
		Contract: testLidoQueueContract,
		Labels:   map[string]string{},
	}

	client1 := &mockExecutionClient{
		name:                       testNodeName1,
		underlyingTokenResponse:    encodeABIAddressReturn("0xabCDEFabcdefABCDefABcdefabcdeFabcdefABCD"),
		symbolResponse:             encodeABIStringReturn("abcETH"),
		decimalsResponse:           encodeABIUintReturn(18),
		withdrawalRequestsResponse: encodeABIUintArrayReturn(1),
		withdrawalStatusResponse:   encodeLidoWithdrawalStatusesReturn(testWithdrawalStatus(1e18, false, false)),
	}

	client2 := &mockExecutionClient{
		name:                       "node-2",
		underlyingTokenResponse:    encodeABIAddressReturn("0xabCDEFabcdefABCDefABcdefabcdeFabcdefABCD"),
		symbolResponse:             encodeABIStringReturn("abcETH"),
		decimalsResponse:           encodeABIUintReturn(18),
		withdrawalRequestsResponse: encodeABIUintArrayReturn(1),
		withdrawalStatusResponse:   encodeLidoWithdrawalStatusesReturn(testWithdrawalStatus(1e18, false, false)),
	}

	queue := NewLidoWithdrawalQueueERC721(
		[]api.ExecutionClient{client1, client2},
		testLogger(),
		15*time.Second,
		"lido_queue_multi",
		map[string]string{},
		[]*AddressLidoWithdrawalQueueERC721{address},
	)

	queue.tick(context.Background())

	if len(client1.callLog) != 5 {
		t.Fatalf("node-1 expected 5 calls, got %d", len(client1.callLog))
	}

	if len(client2.callLog) != 5 {
		t.Fatalf("node-2 expected 5 calls, got %d", len(client2.callLog))
	}
}

func TestLidoWithdrawalQueueERC721_getLabelValues(t *testing.T) {
	addresses := []*AddressLidoWithdrawalQueueERC721{
		{
			Name:     "Label Queue",
			Address:  testLidoHolderAddress,
			Contract: testLidoQueueContract,
			Labels: map[string]string{
				testLabelKeyType: "withdrawal",
			},
		},
	}

	queue := NewLidoWithdrawalQueueERC721(
		mockClients(&mockExecutionClient{}),
		testLogger(),
		15*time.Second,
		"lido_queue_labels",
		map[string]string{},
		addresses,
	)

	labels := queue.getLabelValues(addresses[0], "abcETH", "mock-node")
	assertLabelContains(t, labels, "Label Queue")
	assertLabelContains(t, labels, testLidoHolderAddress)
	assertLabelContains(t, labels, testLidoQueueContract)
	assertLabelContains(t, labels, "abcETH")
	assertLabelContains(t, labels, "mock-node")
	assertLabelContains(t, labels, "withdrawal")
}

func TestLidoWithdrawalQueueERC721_Name(t *testing.T) {
	queue := &LidoWithdrawalQueueERC721{}
	if queue.Name() != NameLidoWithdrawalQueueERC721 {
		t.Errorf("Expected name %s, got %s", NameLidoWithdrawalQueueERC721, queue.Name())
	}
}

func assertMetricValue(t *testing.T, metric prometheus.GaugeVec, labels []string, expected float64) {
	t.Helper()

	value := testutil.ToFloat64(metric.WithLabelValues(labels...))
	if diff := value - expected; diff > 0.000001 || diff < -0.000001 {
		t.Fatalf("metric value = %f, want %f", value, expected)
	}
}

func assertCallSelectors(t *testing.T, calls []mockCall, selectors []string) {
	t.Helper()

	if len(calls) != len(selectors) {
		t.Fatalf("expected %d calls, got %d", len(selectors), len(calls))
	}

	for i, selector := range selectors {
		if !strings.HasPrefix(calls[i].data, selector) {
			t.Fatalf("call %d selector = %s, want %s", i, calls[i].data[:10], selector)
		}
	}
}

func assertLabelContains(t *testing.T, labels []string, expected string) {
	t.Helper()

	if slices.Contains(labels, expected) {
		return
	}

	t.Fatalf("label values %v do not contain %s", labels, expected)
}

func testLogger() logrus.FieldLogger {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	return log
}

func testWithdrawalStatus(amount int64, finalized, claimed bool) lidoWithdrawalQueueRequestStatus {
	return lidoWithdrawalQueueRequestStatus{
		amount:      big.NewInt(amount),
		isFinalized: finalized,
		isClaimed:   claimed,
	}
}

func encodeABIAddressReturn(address string) string {
	encoded, err := encodeAddressArgument(address)
	if err != nil {
		panic(err)
	}

	return "0x" + encoded
}

func encodeABIStringReturn(symbol string) string {
	hexValue := fmt.Sprintf("%x", symbol)
	paddingLength := (64 - len(hexValue)%64) % 64

	return "0x" +
		formatABIWord(big.NewInt(32)) +
		formatABIWord(big.NewInt(int64(len(symbol)))) +
		hexValue +
		strings.Repeat("0", paddingLength)
}

func encodeABIUintReturn(value int64) string {
	return "0x" + formatABIWord(big.NewInt(value))
}

func encodeABIUintArrayReturn(values ...int64) string {
	var builder strings.Builder

	builder.WriteString("0x")
	builder.WriteString(formatABIWord(big.NewInt(32)))
	builder.WriteString(formatABIWord(big.NewInt(int64(len(values)))))

	for _, value := range values {
		builder.WriteString(formatABIWord(big.NewInt(value)))
	}

	return builder.String()
}

func encodeLidoWithdrawalStatusesReturn(statuses ...lidoWithdrawalQueueRequestStatus) string {
	var builder strings.Builder

	builder.WriteString("0x")
	builder.WriteString(formatABIWord(big.NewInt(32)))
	builder.WriteString(formatABIWord(big.NewInt(int64(len(statuses)))))

	for _, status := range statuses {
		builder.WriteString(formatABIWord(status.amount))
		builder.WriteString(formatABIWord(big.NewInt(0)))
		builder.WriteString(formatABIWord(big.NewInt(0)))
		builder.WriteString(formatABIWord(big.NewInt(0)))
		builder.WriteString(formatBoolWord(status.isFinalized))
		builder.WriteString(formatBoolWord(status.isClaimed))
	}

	return builder.String()
}

func formatBoolWord(value bool) string {
	if value {
		return formatABIWord(big.NewInt(1))
	}

	return formatABIWord(big.NewInt(0))
}
