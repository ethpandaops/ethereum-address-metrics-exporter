package jobs

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
)

// UniswapPair exposes metrics for ethereum uniswap pair contract.
type UniswapPair struct {
	clients            []api.ExecutionClient
	log                logrus.FieldLogger
	UniswapPairBalance prometheus.GaugeVec
	UniswapPairError   prometheus.CounterVec
	checkInterval      time.Duration
	addresses          []*AddressUniswapPair
	labelsMap          map[string]int
}

type AddressUniswapPair struct {
	From     string            `yaml:"from"`
	To       string            `yaml:"to"`
	Contract string            `yaml:"contract"`
	Name     string            `yaml:"name"`
	Labels   map[string]string `yaml:"labels"`
}

// GetName returns the configured name of this address.
func (a *AddressUniswapPair) GetName() string { return a.Name }

const (
	NameUniswapPair = "uniswap_pair"
)

func (n *UniswapPair) Name() string {
	return NameUniswapPair
}

// NewUniswapPair returns a new UniswapPair instance.
func NewUniswapPair(clients []api.ExecutionClient, log logrus.FieldLogger, checkInterval time.Duration, namespace string, constLabels map[string]string, addresses []*AddressUniswapPair) UniswapPair {
	namespace += "_" + NameUniswapPair

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

	instance := UniswapPair{
		clients:       clients,
		log:           log.WithField("module", NameUniswapPair),
		addresses:     addresses,
		checkInterval: checkInterval,
		labelsMap:     labelsMap,
		UniswapPairBalance: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        metricNameBalance,
				Help:        "The balance of a ethereum uniswap pair contract.",
				ConstLabels: constLabels,
			},
			labels,
		),
		UniswapPairError: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        metricNameErrorsTotal,
				Help:        "The total errors when getting the balance of a ethereum uniswap pair contract.",
				ConstLabels: constLabels,
			},
			labels,
		),
	}

	prometheus.MustRegister(instance.UniswapPairBalance)
	prometheus.MustRegister(instance.UniswapPairError)

	return instance
}

func (n *UniswapPair) Start(ctx context.Context) {
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

func (n *UniswapPair) tick(ctx context.Context) {
	for _, client := range n.clients {
		for _, address := range n.addresses {
			err := n.getBalance(ctx, client, address)
			if err != nil {
				n.log.WithError(err).WithFields(logrus.Fields{
					LabelAddress:   address,
					LabelExecution: client.Name(),
				}).Error("Failed to get uniswap pair balance")
			}
		}
	}
}

func (n *UniswapPair) getLabelValues(address *AddressUniswapPair, executionName string) []string {
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

func (n *UniswapPair) getBalance(ctx context.Context, client api.ExecutionClient, address *AddressUniswapPair) error {
	var err error

	defer func() {
		if err != nil {
			n.UniswapPairError.WithLabelValues(n.getLabelValues(address, client.Name())...).Inc()
		}
	}()

	// call getReserves() which is 0x0902f1ac
	getReservesData := "0x0902f1ac000000000000000000000000"

	balanceStr, err := client.ETHCall(ctx, &api.ETHCallTransaction{
		To:   address.Contract,
		Data: &getReservesData,
	}, "latest")
	if err != nil {
		return err
	}

	if len(balanceStr) < 130 {
		n.log.WithFields(logrus.Fields{
			LabelAddress:      address,
			metricNameBalance: balanceStr,
		}).Warn("Got empty uniswap pair balance")

		return nil
	}

	fromBalance := hexStringToFloat64(balanceStr[0:66])
	toBalance := hexStringToFloat64("0x" + balanceStr[66:130])

	balance := toBalance / fromBalance
	n.UniswapPairBalance.WithLabelValues(n.getLabelValues(address, client.Name())...).Set(balance)

	return nil
}
