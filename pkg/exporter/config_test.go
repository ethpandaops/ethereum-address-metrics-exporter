package exporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/jobs"
)

func TestConfig_Validate_ExecutionNodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		execution []*ExecutionNode
		wantErr   string
	}{
		{
			name:      "no execution nodes",
			execution: nil,
			wantErr:   "at least one execution node must be configured",
		},
		{
			name:      "empty execution list",
			execution: []*ExecutionNode{},
			wantErr:   "at least one execution node must be configured",
		},
		{
			name: "missing name",
			execution: []*ExecutionNode{
				{Name: "", URL: testNodeURL},
			},
			wantErr: "execution node at index 0 must have a name",
		},
		{
			name: "duplicate names",
			execution: []*ExecutionNode{
				{Name: testNodeName1, URL: testNodeURL},
				{Name: testNodeName1, URL: "http://localhost:8546"},
			},
			wantErr: "duplicate execution node with the same name: node-1",
		},
		{
			name: "valid single node",
			execution: []*ExecutionNode{
				{Name: "geth-1", URL: testNodeURL, Timeout: 10 * time.Second},
			},
		},
		{
			name: "valid multiple nodes",
			execution: []*ExecutionNode{
				{Name: "geth-1", URL: testNodeURL},
				{Name: "nethermind-1", URL: "http://localhost:8546"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{
				Execution: tt.execution,
			}

			err := cfg.Validate()

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_DuplicateAddressNames(t *testing.T) {
	t.Parallel()

	validExecution := []*ExecutionNode{
		{Name: testNodeName1, URL: testNodeURL},
	}

	tests := []struct {
		name      string
		addresses Addresses
		wantErr   string
	}{
		{
			name:      "empty addresses passes",
			addresses: Addresses{},
		},
		{
			name: "duplicate account names",
			addresses: Addresses{
				Account: []*jobs.AddressAccount{
					{Name: "dup", Address: testHolder1Address},
					{Name: "dup", Address: testHolder2Address},
				},
			},
			wantErr: "duplicate account address with the same name: dup",
		},
		{
			name: "duplicate erc20 names",
			addresses: Addresses{
				ERC20: []*jobs.AddressERC20{
					{Name: "token", Address: testHolder1Address, Contract: "0xaaaa"},
					{Name: "token", Address: testHolder2Address, Contract: "0xbbbb"},
				},
			},
			wantErr: "duplicate erc20 address with the same name: token",
		},
		{
			name: "duplicate erc4337 names",
			addresses: Addresses{
				ERC4337: []*jobs.AddressERC4337{
					{Name: "paymaster", Address: testHolder1Address, Contract: "0xaaaa"},
					{Name: "paymaster", Address: testHolder2Address, Contract: "0xbbbb"},
				},
			},
			wantErr: "duplicate erc4337 address with the same name: paymaster",
		},
		{
			name: "duplicate lido withdrawal queue erc721 names",
			addresses: Addresses{
				LidoWithdrawalQueueERC721: []*jobs.AddressLidoWithdrawalQueueERC721{
					{Name: "queue", Address: testHolder1Address, Contract: "0xaaaa"},
					{Name: "queue", Address: testHolder2Address, Contract: "0xbbbb"},
				},
			},
			wantErr: "duplicate lido withdrawal queue erc721 address with the same name: queue",
		},
		{
			name: "same name across different types is OK",
			addresses: Addresses{
				Account: []*jobs.AddressAccount{
					{Name: "shared-name", Address: testHolder1Address},
				},
				ERC20: []*jobs.AddressERC20{
					{Name: "shared-name", Address: testHolder2Address, Contract: "0xaaaa"},
				},
			},
		},
		{
			name: "unique names within each type passes",
			addresses: Addresses{
				Account: []*jobs.AddressAccount{
					{Name: "account-1", Address: testHolder1Address},
					{Name: "account-2", Address: testHolder2Address},
				},
				ERC20: []*jobs.AddressERC20{
					{Name: "token-1", Address: "0x3333333333333333333333333333333333333333", Contract: "0xaaaa"},
					{Name: "token-2", Address: "0x4444444444444444444444444444444444444444", Contract: "0xbbbb"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{
				Execution: validExecution,
				Addresses: tt.addresses,
			}

			err := cfg.Validate()

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
