package jobs

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
)

const (
	NameLidoWithdrawalQueueERC721 = "lido_withdrawal_queue_erc721"

	lidoWithdrawalQueueUnderlyingTokenSelector        = "0xe00bfe50" //nolint:gosec // Ethereum ABI selector, not a credential.
	lidoWithdrawalQueueSymbolSelector                 = "0x95d89b41"
	lidoWithdrawalQueueDecimalsSelector               = "0x313ce567"
	lidoWithdrawalQueueGetWithdrawalRequestsSelector  = "0x7d031b65"
	lidoWithdrawalQueueGetWithdrawalStatusSelector    = "0xb8c4b85a"
	lidoWithdrawalQueueRequestStatusWordLength        = 6
	lidoWithdrawalQueueRequestStatusAmountWord        = 0
	lidoWithdrawalQueueRequestStatusFinalizedWord     = 4
	lidoWithdrawalQueueRequestStatusClaimedWord       = 5
	lidoWithdrawalQueueMaxSupportedUnderlyingDecimals = 255
)

// LidoWithdrawalQueueERC721 exposes metrics for Lido-compatible withdrawal queue ERC721 contracts.
type LidoWithdrawalQueueERC721 struct {
	clients       []api.ExecutionClient
	log           logrus.FieldLogger
	checkInterval time.Duration
	addresses     []*AddressLidoWithdrawalQueueERC721
	labelsMap     map[string]int

	LidoWithdrawalQueueERC721RequestCount prometheus.GaugeVec
	LidoWithdrawalQueueERC721Pending      prometheus.GaugeVec
	LidoWithdrawalQueueERC721Claimable    prometheus.GaugeVec
	LidoWithdrawalQueueERC721Claimed      prometheus.GaugeVec
	LidoWithdrawalQueueERC721Error        prometheus.CounterVec
}

type AddressLidoWithdrawalQueueERC721 struct {
	Address  string            `yaml:"address"`
	Contract string            `yaml:"contract"`
	Name     string            `yaml:"name"`
	Labels   map[string]string `yaml:"labels"`
}

// GetName returns the configured name of this address.
func (a *AddressLidoWithdrawalQueueERC721) GetName() string { return a.Name }

func (n *LidoWithdrawalQueueERC721) Name() string {
	return NameLidoWithdrawalQueueERC721
}

// NewLidoWithdrawalQueueERC721 returns a new LidoWithdrawalQueueERC721 instance.
func NewLidoWithdrawalQueueERC721(clients []api.ExecutionClient, log logrus.FieldLogger, checkInterval time.Duration, namespace string, constLabels map[string]string, addresses []*AddressLidoWithdrawalQueueERC721) LidoWithdrawalQueueERC721 {
	namespace += "_" + NameLidoWithdrawalQueueERC721

	labelsMap := map[string]int{
		LabelName:      0,
		LabelAddress:   1,
		LabelContract:  2,
		LabelSymbol:    3,
		LabelExecution: 4,
	}

	for address := range addresses {
		for label := range addresses[address].Labels {
			if _, ok := labelsMap[label]; !ok {
				labelsMap[label] = len(labelsMap)
			}
		}
	}

	labels := make([]string, len(labelsMap))
	for label, index := range labelsMap {
		labels[index] = label
	}

	instance := LidoWithdrawalQueueERC721{
		clients:       clients,
		log:           log.WithField("module", NameLidoWithdrawalQueueERC721),
		addresses:     addresses,
		checkInterval: checkInterval,
		labelsMap:     labelsMap,
		LidoWithdrawalQueueERC721RequestCount: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "request_count",
				Help:        "The number of unclaimed withdrawal queue ERC721 requests owned by an address.",
				ConstLabels: constLabels,
			},
			labels,
		),
		LidoWithdrawalQueueERC721Pending: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "pending",
				Help:        "The pending underlying token amount in withdrawal queue ERC721 requests owned by an address.",
				ConstLabels: constLabels,
			},
			labels,
		),
		LidoWithdrawalQueueERC721Claimable: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "claimable",
				Help:        "The claimable underlying token amount in withdrawal queue ERC721 requests owned by an address.",
				ConstLabels: constLabels,
			},
			labels,
		),
		LidoWithdrawalQueueERC721Claimed: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "claimed",
				Help:        "The claimed underlying token amount in returned withdrawal queue ERC721 request statuses.",
				ConstLabels: constLabels,
			},
			labels,
		),
		LidoWithdrawalQueueERC721Error: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        metricNameErrorsTotal,
				Help:        "The total errors when calling Lido-compatible withdrawal queue ERC721 functions.",
				ConstLabels: constLabels,
			},
			labels,
		),
	}

	prometheus.MustRegister(instance.LidoWithdrawalQueueERC721RequestCount)
	prometheus.MustRegister(instance.LidoWithdrawalQueueERC721Pending)
	prometheus.MustRegister(instance.LidoWithdrawalQueueERC721Claimable)
	prometheus.MustRegister(instance.LidoWithdrawalQueueERC721Claimed)
	prometheus.MustRegister(instance.LidoWithdrawalQueueERC721Error)

	return instance
}

func (n *LidoWithdrawalQueueERC721) Start(ctx context.Context) {
	n.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(n.checkInterval):
			n.tick(ctx)
		}
	}
}

func (n *LidoWithdrawalQueueERC721) tick(ctx context.Context) {
	for _, client := range n.clients {
		for _, address := range n.addresses {
			err := n.getWithdrawalQueue(ctx, client, address)
			if err != nil {
				n.log.WithError(err).WithFields(logrus.Fields{
					LabelAddress:   address,
					LabelExecution: client.Name(),
				}).Error("Failed to get Lido withdrawal queue ERC721 metrics")
			}
		}
	}
}

func (n *LidoWithdrawalQueueERC721) getLabelValues(address *AddressLidoWithdrawalQueueERC721, symbol, executionName string) []string {
	values := make([]string, len(n.labelsMap))

	for label, index := range n.labelsMap {
		if address.Labels != nil && address.Labels[label] != "" {
			values[index] = address.Labels[label]
		} else {
			switch label {
			case LabelName:
				values[index] = address.Name
			case LabelAddress:
				values[index] = address.Address
			case LabelContract:
				values[index] = address.Contract
			case LabelSymbol:
				values[index] = symbol
			case LabelExecution:
				values[index] = executionName
			default:
				values[index] = LabelDefaultValue
			}
		}
	}

	return values
}

func (n *LidoWithdrawalQueueERC721) getWithdrawalQueue(ctx context.Context, client api.ExecutionClient, address *AddressLidoWithdrawalQueueERC721) error {
	var err error

	symbol := ""

	defer func() {
		if err != nil {
			n.LidoWithdrawalQueueERC721Error.WithLabelValues(n.getLabelValues(address, symbol, client.Name())...).Inc()
		}
	}()

	underlyingToken, err := n.getUnderlyingToken(ctx, client, address)
	if err != nil {
		return err
	}

	symbol, err = n.getTokenSymbol(ctx, client, underlyingToken)
	if err != nil {
		return err
	}

	decimals, err := n.getTokenDecimals(ctx, client, underlyingToken)
	if err != nil {
		return err
	}

	requestIDs, err := n.getWithdrawalRequests(ctx, client, address)
	if err != nil {
		return err
	}

	labels := n.getLabelValues(address, symbol, client.Name())
	n.LidoWithdrawalQueueERC721RequestCount.WithLabelValues(labels...).Set(float64(len(requestIDs)))

	if len(requestIDs) == 0 {
		n.setAmounts(labels, 0, 0, 0)

		return nil
	}

	statuses, err := n.getWithdrawalStatuses(ctx, client, address, requestIDs)
	if err != nil {
		return err
	}

	if len(statuses) != len(requestIDs) {
		return fmt.Errorf("got %d withdrawal statuses for %d request ids", len(statuses), len(requestIDs))
	}

	pending, claimable, claimed := sumLidoWithdrawalQueueStatuses(statuses, decimals)
	n.setAmounts(labels, pending, claimable, claimed)

	return nil
}

func (n *LidoWithdrawalQueueERC721) setAmounts(labels []string, pending, claimable, claimed float64) {
	n.LidoWithdrawalQueueERC721Pending.WithLabelValues(labels...).Set(pending)
	n.LidoWithdrawalQueueERC721Claimable.WithLabelValues(labels...).Set(claimable)
	n.LidoWithdrawalQueueERC721Claimed.WithLabelValues(labels...).Set(claimed)
}

func (n *LidoWithdrawalQueueERC721) getUnderlyingToken(ctx context.Context, client api.ExecutionClient, address *AddressLidoWithdrawalQueueERC721) (string, error) {
	callData := lidoWithdrawalQueueUnderlyingTokenSelector

	tokenHex, err := client.ETHCall(ctx, &api.ETHCallTransaction{
		To:   address.Contract,
		Data: &callData,
	}, "latest")
	if err != nil {
		return "", err
	}

	return decodeABIAddress(tokenHex)
}

func (n *LidoWithdrawalQueueERC721) getTokenSymbol(ctx context.Context, client api.ExecutionClient, tokenAddress string) (string, error) {
	callData := lidoWithdrawalQueueSymbolSelector

	symbolHex, err := client.ETHCall(ctx, &api.ETHCallTransaction{
		To:   tokenAddress,
		Data: &callData,
	}, "latest")
	if err != nil {
		return "", err
	}

	return hexStringToString(symbolHex)
}

func (n *LidoWithdrawalQueueERC721) getTokenDecimals(ctx context.Context, client api.ExecutionClient, tokenAddress string) (int, error) {
	callData := lidoWithdrawalQueueDecimalsSelector

	decimalsHex, err := client.ETHCall(ctx, &api.ETHCallTransaction{
		To:   tokenAddress,
		Data: &callData,
	}, "latest")
	if err != nil {
		return 0, err
	}

	decimals, err := decodeABIUint256(decimalsHex)
	if err != nil {
		return 0, err
	}

	if !decimals.IsUint64() || decimals.Uint64() > lidoWithdrawalQueueMaxSupportedUnderlyingDecimals {
		return 0, fmt.Errorf("unsupported underlying token decimals: %s", decimals.String())
	}

	decimalsInt := 0
	for range decimals.Uint64() {
		decimalsInt++
	}

	return decimalsInt, nil
}

func (n *LidoWithdrawalQueueERC721) getWithdrawalRequests(ctx context.Context, client api.ExecutionClient, address *AddressLidoWithdrawalQueueERC721) ([]*big.Int, error) {
	callData, err := encodeAddressCall(lidoWithdrawalQueueGetWithdrawalRequestsSelector, address.Address)
	if err != nil {
		return nil, err
	}

	requestsHex, err := client.ETHCall(ctx, &api.ETHCallTransaction{
		To:   address.Contract,
		Data: &callData,
	}, "latest")
	if err != nil {
		return nil, err
	}

	return decodeABIUint256Array(requestsHex)
}

func (n *LidoWithdrawalQueueERC721) getWithdrawalStatuses(ctx context.Context, client api.ExecutionClient, address *AddressLidoWithdrawalQueueERC721, requestIDs []*big.Int) ([]lidoWithdrawalQueueRequestStatus, error) {
	callData := encodeUint256ArrayCall(lidoWithdrawalQueueGetWithdrawalStatusSelector, requestIDs)

	statusesHex, err := client.ETHCall(ctx, &api.ETHCallTransaction{
		To:   address.Contract,
		Data: &callData,
	}, "latest")
	if err != nil {
		return nil, err
	}

	return decodeLidoWithdrawalQueueStatuses(statusesHex)
}

type lidoWithdrawalQueueRequestStatus struct {
	amount      *big.Int
	isFinalized bool
	isClaimed   bool
}

func sumLidoWithdrawalQueueStatuses(statuses []lidoWithdrawalQueueRequestStatus, decimals int) (float64, float64, float64) {
	pending := new(big.Int)
	claimable := new(big.Int)
	claimed := new(big.Int)

	for _, status := range statuses {
		switch {
		case status.isClaimed:
			claimed.Add(claimed, status.amount)
		case status.isFinalized:
			claimable.Add(claimable, status.amount)
		default:
			pending.Add(pending, status.amount)
		}
	}

	return tokenAmountToFloat64(pending, decimals),
		tokenAmountToFloat64(claimable, decimals),
		tokenAmountToFloat64(claimed, decimals)
}

func tokenAmountToFloat64(amount *big.Int, decimals int) float64 {
	if amount.Sign() == 0 {
		return 0
	}

	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	value := new(big.Float).SetPrec(256).SetInt(amount)
	value.Quo(value, new(big.Float).SetPrec(256).SetInt(denominator))

	result, _ := value.Float64()

	return result
}

func encodeAddressCall(selector, address string) (string, error) {
	encodedAddress, err := encodeAddressArgument(address)
	if err != nil {
		return "", err
	}

	return selector + encodedAddress, nil
}

func encodeAddressArgument(address string) (string, error) {
	cleanedAddress := strings.TrimPrefix(strings.ToLower(address), "0x")
	if len(cleanedAddress) != 40 {
		return "", fmt.Errorf("invalid ethereum address length: %s", address)
	}

	if _, err := hex.DecodeString(cleanedAddress); err != nil {
		return "", fmt.Errorf("invalid ethereum address: %w", err)
	}

	return strings.Repeat("0", 24) + cleanedAddress, nil
}

func encodeUint256ArrayCall(selector string, values []*big.Int) string {
	var data strings.Builder

	data.WriteString(selector)
	data.WriteString(formatABIWord(big.NewInt(32)))
	data.WriteString(formatABIWord(big.NewInt(int64(len(values)))))

	for _, value := range values {
		data.WriteString(formatABIWord(value))
	}

	return data.String()
}

func formatABIWord(value *big.Int) string {
	return fmt.Sprintf("%064x", value)
}

func decodeABIAddress(hexStr string) (string, error) {
	data, err := decodeHexBytes(hexStr)
	if err != nil {
		return "", err
	}

	if len(data) < 32 {
		return "", fmt.Errorf("ABI address response too short: %d bytes", len(data))
	}

	return "0x" + hex.EncodeToString(data[12:32]), nil
}

func decodeABIUint256(hexStr string) (*big.Int, error) {
	data, err := decodeHexBytes(hexStr)
	if err != nil {
		return nil, err
	}

	word, err := abiWord(data, 0)
	if err != nil {
		return nil, err
	}

	return new(big.Int).SetBytes(word), nil
}

func decodeABIUint256Array(hexStr string) ([]*big.Int, error) {
	data, offset, length, err := decodeDynamicABIArrayHeader(hexStr)
	if err != nil {
		return nil, err
	}

	values := make([]*big.Int, length)

	for i := range length {
		word, wordErr := abiWord(data, offset+32+i*32)
		if wordErr != nil {
			return nil, wordErr
		}

		values[i] = new(big.Int).SetBytes(word)
	}

	return values, nil
}

func decodeLidoWithdrawalQueueStatuses(hexStr string) ([]lidoWithdrawalQueueRequestStatus, error) {
	data, offset, length, err := decodeDynamicABIArrayHeader(hexStr)
	if err != nil {
		return nil, err
	}

	statuses := make([]lidoWithdrawalQueueRequestStatus, length)

	for i := range length {
		baseOffset := offset + 32 + i*lidoWithdrawalQueueRequestStatusWordLength*32
		amount, amountErr := decodeABIWordAsBigInt(data, baseOffset+lidoWithdrawalQueueRequestStatusAmountWord*32)
		if amountErr != nil {
			return nil, amountErr
		}

		isFinalized, finalizedErr := decodeABIWordAsBool(data, baseOffset+lidoWithdrawalQueueRequestStatusFinalizedWord*32)
		if finalizedErr != nil {
			return nil, finalizedErr
		}

		isClaimed, claimedErr := decodeABIWordAsBool(data, baseOffset+lidoWithdrawalQueueRequestStatusClaimedWord*32)
		if claimedErr != nil {
			return nil, claimedErr
		}

		statuses[i] = lidoWithdrawalQueueRequestStatus{
			amount:      amount,
			isFinalized: isFinalized,
			isClaimed:   isClaimed,
		}
	}

	return statuses, nil
}

func decodeDynamicABIArrayHeader(hexStr string) ([]byte, int, int, error) {
	data, err := decodeHexBytes(hexStr)
	if err != nil {
		return nil, 0, 0, err
	}

	offset, err := decodeABIWordAsInt(data, 0)
	if err != nil {
		return nil, 0, 0, err
	}

	length, err := decodeABIWordAsInt(data, offset)
	if err != nil {
		return nil, 0, 0, err
	}

	return data, offset, length, nil
}

func decodeABIWordAsBigInt(data []byte, offset int) (*big.Int, error) {
	word, err := abiWord(data, offset)
	if err != nil {
		return nil, err
	}

	return new(big.Int).SetBytes(word), nil
}

func decodeABIWordAsBool(data []byte, offset int) (bool, error) {
	value, err := decodeABIWordAsBigInt(data, offset)
	if err != nil {
		return false, err
	}

	return value.Sign() != 0, nil
}

func decodeABIWordAsInt(data []byte, offset int) (int, error) {
	value, err := decodeABIWordAsBigInt(data, offset)
	if err != nil {
		return 0, err
	}

	if !value.IsInt64() {
		return 0, fmt.Errorf("ABI value does not fit in int64: %s", value.String())
	}

	int64Value := value.Int64()
	if int64Value < 0 || int64Value > int64(^uint(0)>>1) {
		return 0, fmt.Errorf("ABI value does not fit in int: %s", value.String())
	}

	return int(int64Value), nil
}

func abiWord(data []byte, offset int) ([]byte, error) {
	if offset < 0 {
		return nil, errors.New("negative ABI word offset")
	}

	if len(data) < offset+32 {
		return nil, fmt.Errorf("ABI word at offset %d exceeds %d bytes", offset, len(data))
	}

	return data[offset : offset+32], nil
}

func decodeHexBytes(hexStr string) ([]byte, error) {
	cleaned := strings.TrimPrefix(hexStr, "0x")
	if cleaned == "" {
		return nil, errors.New("empty hex string")
	}

	if len(cleaned)%2 != 0 {
		cleaned = "0" + cleaned
	}

	return hex.DecodeString(cleaned)
}
