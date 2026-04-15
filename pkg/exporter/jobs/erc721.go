package jobs

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
)

// ERC721 exposes metrics for ethereum ERC721 contract by address.
type ERC721 struct {
	clients       []api.ExecutionClient
	log           logrus.FieldLogger
	ERC721Balance prometheus.GaugeVec
	ERC721Error   prometheus.CounterVec
	checkInterval time.Duration
	addresses     []*AddressERC721
	labelsMap     map[string]int
}

type AddressERC721 struct {
	Address  string            `yaml:"address"`
	Contract string            `yaml:"contract"`
	Name     string            `yaml:"name"`
	Labels   map[string]string `yaml:"labels"`
}

// GetName returns the configured name of this address.
func (a *AddressERC721) GetName() string { return a.Name }

const (
	NameERC721 = "erc721"
)

func (n *ERC721) Name() string {
	return NameERC721
}

// NewERC721 returns a new ERC721 instance.
func NewERC721(clients []api.ExecutionClient, log logrus.FieldLogger, checkInterval time.Duration, namespace string, constLabels map[string]string, addresses []*AddressERC721) ERC721 {
	namespace += "_" + NameERC721

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

	instance := ERC721{
		clients:       clients,
		log:           log.WithField("module", NameERC721),
		addresses:     addresses,
		checkInterval: checkInterval,
		labelsMap:     labelsMap,
		ERC721Balance: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "balance",
				Help:        "The balance of a ethereum ERC721 contract by address.",
				ConstLabels: constLabels,
			},
			labels,
		),
		ERC721Error: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        "errors_total",
				Help:        "The total errors when getting the balance of a ethereum ERC721 contract by address.",
				ConstLabels: constLabels,
			},
			labels,
		),
	}

	prometheus.MustRegister(instance.ERC721Balance)
	prometheus.MustRegister(instance.ERC721Error)

	return instance
}

func (n *ERC721) Start(ctx context.Context) {
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

func (n *ERC721) tick(ctx context.Context) {
	for _, client := range n.clients {
		for _, address := range n.addresses {
			err := n.getBalance(ctx, client, address)
			if err != nil {
				n.log.WithError(err).WithFields(logrus.Fields{
					"address":   address,
					"execution": client.Name(),
				}).Error("Failed to get erc721 contract balanceOf address")
			}
		}
	}
}

func (n *ERC721) getLabelValues(address *AddressERC721, executionName string) []string {
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

func (n *ERC721) getBalance(ctx context.Context, client api.ExecutionClient, address *AddressERC721) error {
	var err error

	defer func() {
		if err != nil {
			n.ERC721Error.WithLabelValues(n.getLabelValues(address, client.Name())...).Inc()
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

	n.ERC721Balance.WithLabelValues(n.getLabelValues(address, client.Name())...).Set(hexStringToFloat64(balanceStr))

	return nil
}
