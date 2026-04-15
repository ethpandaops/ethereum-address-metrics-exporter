package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
)

// ERC4626 exposes metrics for ethereum ERC4626 vault contracts.
type ERC4626 struct {
	clients       []api.ExecutionClient
	log           logrus.FieldLogger
	ERC4626Assets prometheus.GaugeVec
	ERC4626Error  prometheus.CounterVec
	checkInterval time.Duration
	addresses     []*AddressERC4626
	labelsMap     map[string]int
}

type AddressERC4626 struct {
	Address  string            `yaml:"address"`
	Contract string            `yaml:"contract"`
	Name     string            `yaml:"name"`
	Labels   map[string]string `yaml:"labels"`
}

// GetName returns the configured name of this address.
func (a *AddressERC4626) GetName() string { return a.Name }

const (
	NameERC4626 = "erc4626"
)

func (n *ERC4626) Name() string {
	return NameERC4626
}

// NewERC4626 returns a new ERC4626 instance.
func NewERC4626(clients []api.ExecutionClient, log logrus.FieldLogger, checkInterval time.Duration, namespace string, constLabels map[string]string, addresses []*AddressERC4626) ERC4626 {
	namespace += "_" + NameERC4626

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

	instance := ERC4626{
		clients:       clients,
		log:           log.WithField("module", NameERC4626),
		addresses:     addresses,
		checkInterval: checkInterval,
		labelsMap:     labelsMap,
		ERC4626Assets: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "assets",
				Help:        "The asset value from ERC4626 vault convertToAssets function.",
				ConstLabels: constLabels,
			},
			labels,
		),
		ERC4626Error: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        "errors_total",
				Help:        "The total errors when calling ERC4626 vault functions.",
				ConstLabels: constLabels,
			},
			labels,
		),
	}

	prometheus.MustRegister(instance.ERC4626Assets)
	prometheus.MustRegister(instance.ERC4626Error)

	return instance
}

func (n *ERC4626) Start(ctx context.Context) {
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

func (n *ERC4626) tick(ctx context.Context) {
	for _, client := range n.clients {
		for _, address := range n.addresses {
			err := n.getAssets(ctx, client, address)
			if err != nil {
				n.log.WithError(err).WithFields(logrus.Fields{
					"address":   address,
					"execution": client.Name(),
				}).Error("Failed to get ERC4626 vault assets")
			}
		}
	}
}

func (n *ERC4626) getLabelValues(address *AddressERC4626, symbol, executionName string) []string {
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

func (n *ERC4626) getAssets(ctx context.Context, client api.ExecutionClient, address *AddressERC4626) error {
	var err error

	symbol := ""

	defer func() {
		if err != nil {
			n.ERC4626Error.WithLabelValues(n.getLabelValues(address, symbol, client.Name())...).Inc()
		}
	}()

	// Step 1: Call balanceOf(address) on the vault contract to get shares balance
	// Function selector for balanceOf(address) is 0x70a08231
	balanceOfData := "0x70a08231000000000000000000000000" + address.Address[2:]

	sharesStr, err := client.ETHCall(ctx, &api.ETHCallTransaction{
		To:   address.Contract,
		Data: &balanceOfData,
	}, "latest")
	if err != nil {
		return err
	}

	// Extract the shares value (remove 0x prefix and ensure proper padding)
	shares := strings.TrimPrefix(sharesStr, "0x")
	if shares == "" {
		shares = "0"
	}

	// Ensure shares is properly padded to 32 bytes (64 hex chars)
	if len(shares) < 64 {
		shares = fmt.Sprintf("%064s", shares)
	}

	// Step 2: Call convertToAssets(uint256 shares) on the vault contract
	// Function selector for convertToAssets(uint256) is 0x07a2d13a
	convertToAssetsData := "0x07a2d13a" + shares

	assetsStr, err := client.ETHCall(ctx, &api.ETHCallTransaction{
		To:   address.Contract,
		Data: &convertToAssetsData,
	}, "latest")
	if err != nil {
		return err
	}

	// Step 3: Call symbol() to get the vault token symbol
	// Function selector for symbol() is 0x95d89b41
	symbolData := "0x95d89b41000000000000000000000000"

	symbolHex, err := client.ETHCall(ctx, &api.ETHCallTransaction{
		To:   address.Contract,
		Data: &symbolData,
	}, "latest")
	if err != nil {
		return err
	}

	symbol, err = hexStringToString(symbolHex)
	if err != nil {
		return err
	}

	n.ERC4626Assets.WithLabelValues(n.getLabelValues(address, symbol, client.Name())...).Set(hexStringToFloat64(assetsStr))

	return nil
}
