package jobs

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
)

// ERC4337 exposes metrics for ethereum ERC4337 EntryPoint contract by address.
type ERC4337 struct {
	clients        []api.ExecutionClient
	log            logrus.FieldLogger
	ERC4337Balance prometheus.GaugeVec
	ERC4337Error   prometheus.CounterVec
	checkInterval  time.Duration
	addresses      []*AddressERC4337
	labelsMap      map[string]int
}

type AddressERC4337 struct {
	Address  string            `yaml:"address"`
	Contract string            `yaml:"contract"`
	Name     string            `yaml:"name"`
	Labels   map[string]string `yaml:"labels"`
}

// GetName returns the configured name of this address.
func (a *AddressERC4337) GetName() string { return a.Name }

const (
	NameERC4337 = "erc4337"
)

func (n *ERC4337) Name() string {
	return NameERC4337
}

// NewERC4337 returns a new ERC4337 instance.
func NewERC4337(clients []api.ExecutionClient, log logrus.FieldLogger, checkInterval time.Duration, namespace string, constLabels map[string]string, addresses []*AddressERC4337) ERC4337 {
	namespace += "_" + NameERC4337

	labelsMap := map[string]int{
		LabelName:      0,
		LabelAddress:   1,
		LabelContract:  2,
		LabelExecution: 3,
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

	instance := ERC4337{
		clients:       clients,
		log:           log.WithField("module", NameERC4337),
		addresses:     addresses,
		checkInterval: checkInterval,
		labelsMap:     labelsMap,
		ERC4337Balance: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "balance",
				Help:        "The deposit balance of a ethereum ERC4337 account in the EntryPoint contract.",
				ConstLabels: constLabels,
			},
			labels,
		),
		ERC4337Error: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        "errors_total",
				Help:        "The total errors when getting the deposit balance of a ethereum ERC4337 account.",
				ConstLabels: constLabels,
			},
			labels,
		),
	}

	prometheus.MustRegister(instance.ERC4337Balance)
	prometheus.MustRegister(instance.ERC4337Error)

	return instance
}

func (n *ERC4337) Start(ctx context.Context) {
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

func (n *ERC4337) tick(ctx context.Context) {
	for _, client := range n.clients {
		for _, address := range n.addresses {
			err := n.getBalance(ctx, client, address)
			if err != nil {
				n.log.WithError(err).WithFields(logrus.Fields{
					"address":   address,
					"execution": client.Name(),
				}).Error("Failed to get erc4337 contract balanceOf address")
			}
		}
	}
}

func (n *ERC4337) getLabelValues(address *AddressERC4337, executionName string) []string {
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
			case LabelExecution:
				values[index] = executionName
			default:
				values[index] = LabelDefaultValue
			}
		}
	}

	return values
}

func (n *ERC4337) getBalance(ctx context.Context, client api.ExecutionClient, address *AddressERC4337) error {
	var err error

	defer func() {
		if err != nil {
			n.ERC4337Error.WithLabelValues(n.getLabelValues(address, client.Name())...).Inc()
		}
	}()

	// call balanceOf(address) which is 0x70a08231
	// This is the standard ERC20 balanceOf signature, which EntryPoint also uses for deposits
	balanceOfData := "0x70a08231000000000000000000000000" + address.Address[2:]

	balanceStr, err := client.ETHCall(ctx, &api.ETHCallTransaction{
		To:   address.Contract,
		Data: &balanceOfData,
	}, "latest")
	if err != nil {
		return err
	}

	n.ERC4337Balance.WithLabelValues(n.getLabelValues(address, client.Name())...).Set(hexStringToFloat64(balanceStr))

	return nil
}
