package exporter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/api"
)

// Exporter defines the Ethereum Metrics Exporter interface.
type Exporter interface {
	// Init initialises the exporter
	Start(ctx context.Context) error
}

// NewExporter returns a new Exporter instance.
func NewExporter(log logrus.FieldLogger, conf *Config) Exporter {
	if err := conf.Validate(); err != nil {
		log.Fatalf("invalid config: %s", err)
	}

	return &exporter{
		log: log.WithField("component", "exporter"),
		Cfg: conf,
	}
}

type exporter struct {
	// Helpers
	log logrus.FieldLogger
	Cfg *Config

	clients []api.ExecutionClient
	// Metrics
	metrics Metrics
}

func (e *exporter) Start(ctx context.Context) error {
	e.log.Info("Initializing...")

	e.clients = make([]api.ExecutionClient, 0, len(e.Cfg.Execution))

	httpMetrics := api.NewMetrics(e.Cfg.GlobalConfig.Namespace + "_http")

	for _, node := range e.Cfg.Execution {
		client := api.NewExecutionClient(
			e.log,
			httpMetrics,
			node.Name,
			node.URL,
			node.Headers,
			node.Timeout,
		)

		e.clients = append(e.clients, client)

		e.log.WithFields(logrus.Fields{
			"name": node.Name,
			"url":  node.URL,
		}).Info("Configured execution node")
	}

	e.metrics = NewMetrics(
		e.clients,
		e.log,
		e.Cfg.GlobalConfig.CheckInterval,
		e.Cfg.GlobalConfig.Namespace,
		e.Cfg.GlobalConfig.Labels,
		&e.Cfg.Addresses,
	)

	e.log.Info(fmt.Sprintf("Starting metrics server on %v", e.Cfg.GlobalConfig.MetricsAddr))

	http.Handle("/metrics", promhttp.Handler())

	if err := e.ServeMetrics(ctx); err != nil {
		return err
	}

	go e.metrics.StartAsync(ctx)

	return nil
}

func (e *exporter) ServeMetrics(ctx context.Context) error {
	go func() {
		server := &http.Server{
			Addr:              e.Cfg.GlobalConfig.MetricsAddr,
			ReadHeaderTimeout: 15 * time.Second,
		}

		server.Handler = promhttp.Handler()

		e.log.Infof("Serving metrics at %s", e.Cfg.GlobalConfig.MetricsAddr)

		if err := server.ListenAndServe(); err != nil {
			e.log.Fatal(err)
		}
	}()

	return nil
}
