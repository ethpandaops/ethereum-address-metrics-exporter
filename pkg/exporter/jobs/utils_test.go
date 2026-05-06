package jobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHexStringToFloat64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{
			name:     "1 ETH in wei",
			input:    "0xde0b6b3a7640000",
			expected: 1e18,
		},
		{
			name:     "zero",
			input:    "0x0",
			expected: 0,
		},
		{
			name:     "100 ETH in wei",
			input:    "0x56bc75e2d63100000",
			expected: 1e20,
		},
		{
			name:     "small value",
			input:    "0xa",
			expected: 10,
		},
		{
			name:     "full 32-byte hex",
			input:    "0x0000000000000000000000000000000000000000000000000000000005f5e100",
			expected: 100000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := hexStringToFloat64(tt.input)
			assert.InDelta(t, tt.expected, result, 1.0)
		})
	}
}

func TestHexStringToString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name: "ABI-encoded USDC",
			// offset=32, length=4, data=testNameUSDC
			input:    testABISymbolUSDCResponse,
			expected: testNameUSDC,
		},
		{
			name: "ABI-encoded ETH with length 4",
			// offset=32, length=4, data starts with "UTH\x00" due to hex encoding
			input:    "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000455544800000000000000000000000000000000000000000000000000000000",
			expected: "UTH\x00",
		},
		{
			name: "ABI-encoded sUSDC-Vau with length 9",
			// offset=32, length=9
			input:    testABISymbolSUSDCResponse,
			expected: "sUSDC-Vau",
		},
		{
			name: "ABI-encoded DAI with length 3",
			// offset=32, length=3, data="DAI"
			input:    "0x000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000034441490000000000000000000000000000000000000000000000000000000000",
			expected: "DAI",
		},
		{
			name:    "invalid hex",
			input:   "0xZZZZ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := hexStringToString(tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
