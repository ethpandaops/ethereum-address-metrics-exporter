package jobs

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
)

// ERC20 exposes metrics for ethereum ERC20 contract by address.
type ERC20 struct {
	clients       []api.ExecutionClient
	log           logrus.FieldLogger
	ERC20Balance  prometheus.GaugeVec
	ERC20Error    prometheus.CounterVec
	checkInterval time.Duration
	addresses     []*AddressERC20
	labelsMap     map[string]int
}

type AddressERC20 struct {
	Address  string            `yaml:"address"`
	Contract string            `yaml:"contract"`
	Name     string            `yaml:"name"`
	Labels   map[string]string `yaml:"labels"`
}

// GetName returns the configured name of this address.
func (a *AddressERC20) GetName() string { return a.Name }

const (
	NameERC20 = "erc20"
)

func (n *ERC20) Name() string {
	return NameERC20
}

// NewERC20 returns a new ERC20 instance.
func NewERC20(clients []api.ExecutionClient, log logrus.FieldLogger, checkInterval time.Duration, namespace string, constLabels map[string]string, addresses []*AddressERC20) ERC20 {
	namespace += "_" + NameERC20

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

	instance := ERC20{
		clients:       clients,
		log:           log.WithField("module", NameERC20),
		addresses:     addresses,
		checkInterval: checkInterval,
		labelsMap:     labelsMap,
		ERC20Balance: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        metricNameBalance,
				Help:        "The balance of a ethereum ERC20 contract by address.",
				ConstLabels: constLabels,
			},
			labels,
		),
		ERC20Error: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        metricNameErrorsTotal,
				Help:        "The total errors when getting the balance of a ethereum ERC20 contract by address.",
				ConstLabels: constLabels,
			},
			labels,
		),
	}

	prometheus.MustRegister(instance.ERC20Balance)
	prometheus.MustRegister(instance.ERC20Error)

	return instance
}

func (n *ERC20) Start(ctx context.Context) {
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

func (n *ERC20) tick(ctx context.Context) {
	for _, client := range n.clients {
		for _, address := range n.addresses {
			err := n.getBalance(ctx, client, address)
			if err != nil {
				n.log.WithError(err).WithFields(logrus.Fields{
					LabelAddress:   address,
					LabelExecution: client.Name(),
				}).Error("Failed to get erc20 contract balanceOf address")
			}
		}
	}
}

func (n *ERC20) getLabelValues(address *AddressERC20, symbol, executionName string) []string {
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

func (n *ERC20) getBalance(ctx context.Context, client api.ExecutionClient, address *AddressERC20) error {
	var err error

	symbol := ""

	defer func() {
		if err != nil {
			n.ERC20Error.WithLabelValues(n.getLabelValues(address, symbol, client.Name())...).Inc()
		}
	}()

	// call balanceOf(address) which is 0x70a08231
	balanceOfData := "0x70a08231000000000000000000000000" + address.Address[2:]

	balanceStr, err := client.ETHCall(ctx, &api.ETHCallTransaction{
		To:   address.Contract,
		Data: &balanceOfData,
	}, "latest")
	if err != nil {
		return err
	}

	// call symbol() which is 0x95d89b41
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

	n.ERC20Balance.WithLabelValues(n.getLabelValues(address, symbol, client.Name())...).Set(hexStringToFloat64(balanceStr))

	return nil
}
