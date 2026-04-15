package exporter

import (
	"errors"
	"fmt"
	"time"

	"github.com/ethpandaops/ethereum-address-metrics-exporter/pkg/exporter/jobs"
)

// Config holds the configuration for the ethereum sync status tool.
type Config struct {
	GlobalConfig GlobalConfig `yaml:"global"`
	// Execution is the list of execution nodes to query.
	Execution []*ExecutionNode `yaml:"execution"`
	// Addresses is the list of addresses to monitor.
	Addresses Addresses `yaml:"addresses"`
}

// GlobalConfig holds global configuration settings.
type GlobalConfig struct {
	LoggingLevel  string            `yaml:"logging" default:"warn"`
	MetricsAddr   string            `yaml:"metricsAddr" default:":9090"`
	Namespace     string            `yaml:"namespace" default:"eth_address"`
	CheckInterval time.Duration     `yaml:"checkInterval" default:"15s"`
	Labels        map[string]string `yaml:"labels"`
}

// ExecutionNode represents a single ethereum execution client.
type ExecutionNode struct {
	Name    string            `yaml:"name"`
	URL     string            `yaml:"url" default:"http://localhost:8545"`
	Headers map[string]string `yaml:"headers"`
	Timeout time.Duration     `yaml:"timeout" default:"10s"`
}

// Addresses holds all address types to monitor.
type Addresses struct {
	Account           []*jobs.AddressAccount           `yaml:"account"`
	ERC20             []*jobs.AddressERC20             `yaml:"erc20"`
	ERC721            []*jobs.AddressERC721            `yaml:"erc721"`
	ERC1155           []*jobs.AddressERC1155           `yaml:"erc1155"`
	ERC4626           []*jobs.AddressERC4626           `yaml:"erc4626"`
	UniswapPair       []*jobs.AddressUniswapPair       `yaml:"uniswapPair"`
	ChainlinkDataFeed []*jobs.AddressChainlinkDataFeed `yaml:"chainlinkDataFeed"`
	ERC4337           []*jobs.AddressERC4337           `yaml:"erc4337"`
}

// named is implemented by address types that have a Name field.
type named interface {
	GetName() string
}

// checkDuplicateNames validates that no two entries in a slice share the same name.
func checkDuplicateNames[T named](items []T, typeName string) error {
	seen := make(map[string]struct{}, len(items))

	for _, item := range items {
		name := item.GetName()
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate %s address with the same name: %s", typeName, name)
		}

		seen[name] = struct{}{}
	}

	return nil
}

func (c *Config) Validate() error {
	if len(c.Execution) == 0 {
		return errors.New("at least one execution node must be configured")
	}

	execNames := make(map[string]struct{}, len(c.Execution))

	for i, node := range c.Execution {
		if node.Name == "" {
			return fmt.Errorf("execution node at index %d must have a name", i)
		}

		if _, ok := execNames[node.Name]; ok {
			return fmt.Errorf("duplicate execution node with the same name: %s", node.Name)
		}

		execNames[node.Name] = struct{}{}
	}

	checks := []struct {
		err error
	}{
		{checkDuplicateNames(c.Addresses.Account, "account")},
		{checkDuplicateNames(c.Addresses.ERC20, "erc20")},
		{checkDuplicateNames(c.Addresses.ERC721, "erc721")},
		{checkDuplicateNames(c.Addresses.ERC1155, "erc1155")},
		{checkDuplicateNames(c.Addresses.ERC4626, "erc4626")},
		{checkDuplicateNames(c.Addresses.UniswapPair, "uniswap pair")},
		{checkDuplicateNames(c.Addresses.ChainlinkDataFeed, "chainlink data feed")},
		{checkDuplicateNames(c.Addresses.ERC4337, "erc4337")},
	}

	for _, check := range checks {
		if check.err != nil {
			return check.err
		}
	}

	return nil
}
