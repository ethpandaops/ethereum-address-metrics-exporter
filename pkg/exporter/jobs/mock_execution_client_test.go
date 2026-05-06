package jobs

import (
	"context"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
)

// mockExecutionClient is a mock implementation of the ExecutionClient interface for testing.
type mockExecutionClient struct {
	name                       string
	balanceOfResponse          string
	convertToAssetsResponse    string
	symbolResponse             string
	getReservesResponse        string
	latestAnswerResponse       string
	underlyingTokenResponse    string
	decimalsResponse           string
	withdrawalRequestsResponse string
	withdrawalStatusResponse   string
	balanceOfError             error
	convertToAssetsError       error
	symbolError                error
	callLog                    []mockCall
	ethGetBalanceResponse      string
	ethGetBalanceError         error
	ethGetBalanceCalls         int
}

type mockCall struct {
	to   string
	data string
}

type mockETHCallHandler struct {
	selector string
	handle   func() (string, error)
}

func (m *mockExecutionClient) Name() string {
	if m.name != "" {
		return m.name
	}

	return "mock-node"
}

func (m *mockExecutionClient) ETHCall(_ context.Context, transaction *api.ETHCallTransaction, block string) (string, error) {
	m.callLog = append(m.callLog, mockCall{
		to:   transaction.To,
		data: *transaction.Data,
	})

	for _, handler := range m.ethCallHandlers() {
		if len(*transaction.Data) >= 10 && (*transaction.Data)[:10] == handler.selector {
			return handler.handle()
		}
	}

	return "0x0", nil
}

func (m *mockExecutionClient) ethCallHandlers() []mockETHCallHandler {
	return []mockETHCallHandler{
		{
			selector: "0x70a08231",
			handle:   m.handleBalanceOf,
		},
		{
			selector: "0x07a2d13a",
			handle:   m.handleConvertToAssets,
		},
		{
			selector: "0x95d89b41",
			handle:   m.handleSymbol,
		},
		{
			selector: "0x0902f1ac",
			handle:   m.handleGetReserves,
		},
		{
			selector: "0x50d25bcd",
			handle:   m.handleLatestAnswer,
		},
		{
			selector: "0xe00bfe50",
			handle:   m.handleUnderlyingToken,
		},
		{
			selector: "0x313ce567",
			handle:   m.handleDecimals,
		},
		{
			selector: "0x7d031b65",
			handle:   m.handleWithdrawalRequests,
		},
		{
			selector: "0xb8c4b85a",
			handle:   m.handleWithdrawalStatus,
		},
		{
			selector: "0x00fdd58e",
			handle:   m.handleBalanceOf,
		},
	}
}

func (m *mockExecutionClient) handleBalanceOf() (string, error) {
	if m.balanceOfError != nil {
		return "", m.balanceOfError
	}

	return m.balanceOfResponse, nil
}

func (m *mockExecutionClient) handleConvertToAssets() (string, error) {
	if m.convertToAssetsError != nil {
		return "", m.convertToAssetsError
	}

	return m.convertToAssetsResponse, nil
}

func (m *mockExecutionClient) handleSymbol() (string, error) {
	if m.symbolError != nil {
		return "", m.symbolError
	}

	if m.symbolResponse != "" {
		return m.symbolResponse, nil
	}

	return "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000455544800000000000000000000000000000000000000000000000000000000", nil
}

func (m *mockExecutionClient) handleGetReserves() (string, error) {
	if m.getReservesResponse != "" {
		return m.getReservesResponse, nil
	}

	return "0x0000000000000000000000000000000000000000000000000de0b6b3a76400000000000000000000000000000000000000000000000000000de0b6b3a7640000", nil
}

func (m *mockExecutionClient) handleLatestAnswer() (string, error) {
	if m.latestAnswerResponse != "" {
		return m.latestAnswerResponse, nil
	}

	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

func (m *mockExecutionClient) handleUnderlyingToken() (string, error) {
	if m.underlyingTokenResponse != "" {
		return m.underlyingTokenResponse, nil
	}

	return "0x000000000000000000000000ae7ab96520de3a18e5e111b5eaab095312d7fe84", nil
}

func (m *mockExecutionClient) handleDecimals() (string, error) {
	if m.decimalsResponse != "" {
		return m.decimalsResponse, nil
	}

	return "0x0000000000000000000000000000000000000000000000000000000000000012", nil
}

func (m *mockExecutionClient) handleWithdrawalRequests() (string, error) {
	if m.withdrawalRequestsResponse != "" {
		return m.withdrawalRequestsResponse, nil
	}

	return "0x00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000", nil
}

func (m *mockExecutionClient) handleWithdrawalStatus() (string, error) {
	if m.withdrawalStatusResponse != "" {
		return m.withdrawalStatusResponse, nil
	}

	return "0x00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000", nil
}

func (m *mockExecutionClient) ETHGetBalance(_ context.Context, address string, block string) (string, error) {
	m.ethGetBalanceCalls++

	if m.ethGetBalanceError != nil {
		return "", m.ethGetBalanceError
	}

	if m.ethGetBalanceResponse != "" {
		return m.ethGetBalanceResponse, nil
	}

	return "0x0", nil
}

// mockClients wraps a single mock client in a slice for use with job constructors.
func mockClients(m *mockExecutionClient) []api.ExecutionClient {
	return []api.ExecutionClient{m}
}
