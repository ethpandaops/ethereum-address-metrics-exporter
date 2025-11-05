package jobs

import (
	"bytes"
	"encoding/hex"
	"math/big"
)

const (
	LabelAddress      string = "address"
	LabelContract     string = "contract"
	LabelDefaultValue string = ""
	LabelFrom         string = "from"
	LabelName         string = "name"
	LabelSymbol       string = "symbol"
	LabelTo           string = "to"
	LabelTokenID      string = "token_id"
)

func hexStringToFloat64(hexStr string) float64 {
	f := new(big.Float)
	f.SetString(hexStr)
	balance, _ := f.Float64()

	return balance
}

func hexStringToString(hexStr string) (string, error) {
	bs, err := hex.DecodeString(hexStr[2:])
	if err != nil {
		return "", err
	}

	// ABI-encoded strings have:
	// - First 32 bytes: offset to data location (usually 0x20 = 32)
	// - Next 32 bytes: length of string
	// - Remaining bytes: actual string data (padded to 32-byte boundaries)

	if len(bs) < 64 {
		// Fallback for non-standard encoding
		last := bytes.Trim(bs, "\x00")
		last = bytes.TrimSpace(last)

		return string(last), nil
	}

	// Read the length from bytes 32-64
	lengthBig := new(big.Int).SetBytes(bs[32:64])
	length := lengthBig.Int64()

	if length <= 0 || int(length) > len(bs)-64 {
		// Fallback if length seems invalid
		last := bytes.Trim(bs, "\x00")
		last = bytes.TrimSpace(last)

		return string(last), nil
	}

	// Extract the actual string data starting at byte 64
	stringData := bs[64 : 64+length]

	return string(stringData), nil
}
