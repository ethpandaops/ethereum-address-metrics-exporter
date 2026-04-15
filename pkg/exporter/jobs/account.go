package jobs

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
)

// Account exposes metrics for account addresses.
type Account struct {
	clients        []api.ExecutionClient
	log            logrus.FieldLogger
	AccountBalance prometheus.GaugeVec
	AccountError   prometheus.CounterVec
	checkInterval  time.Duration
	addresses      []*AddressAccount
	labelsMap      map[string]int
}

type AddressAccount struct {
	Address string            `yaml:"address"`
	Name    string            `yaml:"name"`
	Labels  map[string]string `yaml:"labels"`
}

// GetName returns the configured name of this address.
func (a *AddressAccount) GetName() string { return a.Name }

const (
	NameAccount = "account"
)

func (n *Account) Name() string {
	return NameAccount
}

// NewAccount returns a new Account instance.
func NewAccount(clients []api.ExecutionClient, log logrus.FieldLogger, checkInterval time.Duration, namespace string, constLabels map[string]string, addresses []*AddressAccount) Account {
	namespace += "_" + NameAccount

	labelsMap := make(map[string]int, 3)
	labelsMap[LabelName] = 0
	labelsMap[LabelAddress] = 1
	labelsMap[LabelExecution] = 2

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

	instance := Account{
		clients:       clients,
		log:           log.WithField("module", NameAccount),
		addresses:     addresses,
		checkInterval: checkInterval,
		labelsMap:     labelsMap,
		AccountBalance: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace:   namespace,
				Name:        "balance",
				Help:        "The balance of a account address.",
				ConstLabels: constLabels,
			},
			labels,
		),
		AccountError: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   namespace,
				Name:        "errors_total",
				Help:        "The total errors when getting the balance of a account address.",
				ConstLabels: constLabels,
			},
			labels,
		),
	}

	prometheus.MustRegister(instance.AccountBalance)
	prometheus.MustRegister(instance.AccountError)

	return instance
}

func (n *Account) Start(ctx context.Context) {
	n.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(n.checkInterval):
			n.log.WithField("asd", n.checkInterval).Debug("Tick")
			n.tick(ctx)
		}
	}
}

func (n *Account) tick(ctx context.Context) {
	for _, client := range n.clients {
		for _, address := range n.addresses {
			err := n.getBalance(ctx, client, address)
			if err != nil {
				n.log.WithError(err).WithFields(logrus.Fields{
					"address":   address,
					"execution": client.Name(),
				}).Error("Failed to get Account balance")
			}
		}
	}
}

func (n *Account) getLabelValues(address *AddressAccount, executionName string) []string {
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
			case LabelExecution:
				values[index] = executionName
			default:
				values[index] = LabelDefaultValue
			}
		}
	}

	return values
}

func (n *Account) getBalance(ctx context.Context, client api.ExecutionClient, address *AddressAccount) error {
	var err error

	defer func() {
		if err != nil {
			n.AccountError.WithLabelValues(n.getLabelValues(address, client.Name())...).Inc()
		}
	}()

	balance, err := client.ETHGetBalance(ctx, address.Address, "latest")
	if err != nil {
		return err
	}

	balanceFloat64 := hexStringToFloat64(balance)
	n.AccountBalance.WithLabelValues(n.getLabelValues(address, client.Name())...).Set(balanceFloat64)

	return nil
}
