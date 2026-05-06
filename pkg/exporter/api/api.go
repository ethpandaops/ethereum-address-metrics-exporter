package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// ExecutionClient is an interface for executing RPC calls to the Ethereum node.
type ExecutionClient interface {
	// Name returns the configured name of this execution client.
	Name() string
	// ETHCall executes a new message call immediately without creating a transaction on the block chain.
	ETHCall(ctx context.Context, transaction *ETHCallTransaction, block string) (string, error)
	// ETHGetBalance returns the balance of the account of given address.
	ETHGetBalance(ctx context.Context, address string, block string) (string, error)
}

// ETHCallTransaction represents an eth_call transaction object.
type ETHCallTransaction struct {
	From     *string `json:"from"`
	To       string  `json:"to"`
	Gas      *string `json:"gas"`
	GasPrice *string `json:"gasPrice"`
	Value    *string `json:"value"`
	Data     *string `json:"data"`
}

type executionClient struct {
	name    string
	url     string
	path    string
	log     logrus.FieldLogger
	client  http.Client
	headers map[string]string

	metrics Metrics
}

// NewExecutionClient creates a new ExecutionClient. The provided Metrics
// instance is shared across all clients so each call must not register its
// own collectors with Prometheus.
func NewExecutionClient(log logrus.FieldLogger, metrics Metrics, name, rawURL string, headers map[string]string, timeout time.Duration) ExecutionClient {
	client := http.Client{
		Timeout: timeout,
	}

	path := "/"
	if parsed, err := url.Parse(rawURL); err == nil && parsed.Path != "" {
		path = parsed.Path
	}

	return &executionClient{
		name:    name,
		url:     rawURL,
		path:    path,
		log:     log,
		client:  client,
		headers: headers,

		metrics: metrics,
	}
}

// Name returns the configured name of this execution client.
func (e *executionClient) Name() string {
	return e.name
}

type apiResponse struct {
	JSONRpc string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result"`
}

func (e *executionClient) post(ctx context.Context, method string, params any, id int) (json.RawMessage, error) {
	start := time.Now()

	httpMethod := "POST"

	e.metrics.ObserveRequest(httpMethod, e.path, method, e.name)

	var rsp *http.Response

	var err error

	defer func() {
		rspCode := "none"
		if rsp != nil {
			rspCode = strconv.Itoa(rsp.StatusCode)
		}

		e.metrics.ObserveResponse(httpMethod, e.path, method, rspCode, e.name, time.Since(start))
	}()

	body := map[string]any{
		"jsonrpc":   "2.0",
		labelMethod: method,
		"id":        id,
		"params":    params,
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, httpMethod, e.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	req.Header.Set("Content-Type", "application/json")

	rsp, err = e.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", rsp.StatusCode)
	}

	data, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	resp := new(apiResponse)

	if unmarshalErr := json.Unmarshal(data, resp); unmarshalErr != nil {
		return nil, unmarshalErr
	}

	return resp.Result, nil
}

func (e *executionClient) ETHCall(ctx context.Context, transaction *ETHCallTransaction, block string) (string, error) {
	params := []any{
		transaction,
		block,
	}

	rsp, err := e.post(ctx, "eth_call", params, 1)
	if err != nil {
		return "", err
	}

	ethCall := ""

	if unmarshalErr := json.Unmarshal(rsp, &ethCall); unmarshalErr != nil {
		return "", unmarshalErr
	}

	return ethCall, nil
}

func (e *executionClient) ETHGetBalance(ctx context.Context, address, block string) (string, error) {
	params := []any{
		address,
		block,
	}

	rsp, err := e.post(ctx, "eth_getBalance", params, 1)
	if err != nil {
		return "", err
	}

	ethGetBalance := ""

	if unmarshalErr := json.Unmarshal(rsp, &ethGetBalance); unmarshalErr != nil {
		return "", unmarshalErr
	}

	return ethGetBalance, nil
}
