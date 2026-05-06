package jobs

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
)

// ChainlinkDataFeed exposes metrics for ethereum chainlink data feed contract.
type ChainlinkDataFeed struct {
	clients                  []api.ExecutionClient
	log                      logrus.FieldLogger
	ChainlinkDataFeedBalance prometheus.GaugeVec
	ChainlinkDataFeedError   prometheus.CounterVec
	checkInterval            time.Duration
	addresses                []*AddressChainlinkDataFeed
	labelsMap                map[string]int
}

type AddressChainlinkDataFeed struct {
	From     string            `yaml:"from"`
	To       string            `yaml:"to"`
	Contract string            `yaml:"contract"`
	Name     string            `yaml:"name"`
	Labels   map[string]string `yaml:"labels"`
}

// GetName returns the configured name of this address.
func (a *AddressChainlinkDataFeed) GetName() string { return a.Name }

const (
	NameChainlinkDataFeed = "chainlink_data_feed"
)

func (n *ChainlinkDataFeed) Name() string {
	return NameChainlinkDataFeed
}

// NewChainlinkDataFeed returns a new ChainlinkDataFeed instance.
func NewChainlinkDataFeed(clients []api.ExecutionClient, log logrus.FieldLogger, checkInterval time.Duration, namespace string, constLabels map[string]string, addresses []*AddressChainlinkDataFeed) ChainlinkDataFeed {
	namespace += "_" + NameChainlinkDataFeed

	labelsMap := map[string]int{
		LabelName:      0,
		LabelContract:  1,
		LabelFrom:      2,
		LabelTo:        3,
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

	instance := ChainlinkDataFeed{
		clients:       clients,
		log:           log.WithField("module", NameChainlinkDataFeed),
		addresses:     addresses,
		checkInterval: checkInterval,
		labelsMap:     labelsMap,
		ChainlinkDataFeedBalance: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        metricNameBalance,
				Help:        "The balance of a ethereum chainlink data feed contract.",
				ConstLabels: constLabels,
			},
			labels,
		),
		ChainlinkDataFeedError: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        metricNameErrorsTotal,
				Help:        "The total errors when getting the balance of a ethereum chainlink data feed contract.",
				ConstLabels: constLabels,
			},
			labels,
		),
	}

	prometheus.MustRegister(instance.ChainlinkDataFeedBalance)
	prometheus.MustRegister(instance.ChainlinkDataFeedError)

	return instance
}

func (n *ChainlinkDataFeed) Start(ctx context.Context) {
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

func (n *ChainlinkDataFeed) tick(ctx context.Context) {
	for _, client := range n.clients {
		for _, address := range n.addresses {
			err := n.getBalance(ctx, client, address)
			if err != nil {
				n.log.WithError(err).WithFields(logrus.Fields{
					LabelAddress:   address,
					LabelExecution: client.Name(),
				}).Error("Failed to get chainlink data feed balance")
			}
		}
	}
}

func (n *ChainlinkDataFeed) getLabelValues(address *AddressChainlinkDataFeed, executionName string) []string {
	values := make([]string, len(n.labelsMap))

	for label, index := range n.labelsMap {
		if address.Labels != nil && address.Labels[label] != "" {
			values[index] = address.Labels[label]
		} else {
			switch label {
			case LabelName:
				values[index] = address.Name
			case LabelContract:
				values[index] = address.Contract
			case LabelFrom:
				values[index] = address.From
			case LabelTo:
				values[index] = address.To
			case LabelExecution:
				values[index] = executionName
			default:
				values[index] = LabelDefaultValue
			}
		}
	}

	return values
}

func (n *ChainlinkDataFeed) getBalance(ctx context.Context, client api.ExecutionClient, address *AddressChainlinkDataFeed) error {
	var err error

	defer func() {
		if err != nil {
			n.ChainlinkDataFeedError.WithLabelValues(n.getLabelValues(address, client.Name())...).Inc()
		}
	}()

	// call latestAnswer() which is 0x50d25bcd
	latestAnswerData := "0x50d25bcd000000000000000000000000"

	balanceStr, err := client.ETHCall(ctx, &api.ETHCallTransaction{
		To:   address.Contract,
		Data: &latestAnswerData,
	}, "latest")
	if err != nil {
		return err
	}

	n.ChainlinkDataFeedBalance.WithLabelValues(n.getLabelValues(address, client.Name())...).Set(hexStringToFloat64(balanceStr))

	return nil
}
