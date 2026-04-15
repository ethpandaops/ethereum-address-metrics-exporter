package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	ID      int    `json:"id"`
	Params  []any  `json:"params"`
}

var testClientCounter atomic.Int64

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

func newTestClient(t *testing.T, name, url string) ExecutionClient {
	t.Helper()

	n := testClientCounter.Add(1)

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	namespace := "test_" + strconv.FormatInt(n, 10)

	return NewExecutionClient(log, NewMetrics(namespace), name, url, nil, 5*time.Second)
}

func TestExecutionClient_Name(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, "geth-1", "http://unused")
	assert.Equal(t, "geth-1", client.Name())
}

func TestExecutionClient_ETHGetBalance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantResult string
		wantErr    bool
	}{
		{
			name: "successful balance response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)

				var req rpcRequest
				if err := json.Unmarshal(body, &req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0xde0b6b3a7640000"}`))
			},
			wantResult: "0xde0b6b3a7640000",
		},
		{
			name: "http 500 error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
		{
			name: "malformed json response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{invalid json`))
			},
			wantErr: true,
		},
		{
			name: "empty result",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x0"}`))
			},
			wantResult: "0x0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t, tt.handler)
			client := newTestClient(t, "test-node", server.URL)

			result, err := client.ETHGetBalance(context.Background(), "0x1234567890123456789012345678901234567890", "latest")

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantResult, result)
			}
		})
	}
}

func TestExecutionClient_ETHCall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantResult string
		wantErr    bool
	}{
		{
			name: "successful eth_call",
			handler: func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)

				var req rpcRequest
				if err := json.Unmarshal(body, &req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				assert.Equal(t, "eth_call", req.Method)

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x0000000000000000000000000000000000000000000000000000000005f5e100"}`))
			},
			wantResult: "0x0000000000000000000000000000000000000000000000000000000005f5e100",
		},
		{
			name: "http 400 bad request",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t, tt.handler)
			client := newTestClient(t, "test-node", server.URL)

			data := "0x70a08231000000000000000000000000aabbccdd"
			result, err := client.ETHCall(context.Background(), &ETHCallTransaction{
				To:   "0xcontract",
				Data: &data,
			}, "latest")

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantResult, result)
			}
		})
	}
}

func TestExecutionClient_CustomHeaders(t *testing.T) {
	t.Parallel()

	var receivedAuth string

	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x0"}`))
	})

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	n := testClientCounter.Add(1)
	namespace := "test_headers_" + strconv.FormatInt(n, 10)

	headers := map[string]string{
		"Authorization": "Bearer test-token",
	}

	client := NewExecutionClient(log, NewMetrics(namespace), "node-1", server.URL, headers, 5*time.Second)

	_, err := client.ETHGetBalance(context.Background(), "0x1234567890123456789012345678901234567890", "latest")
	require.NoError(t, err)
	assert.Equal(t, "Bearer test-token", receivedAuth)
}

func TestExecutionClient_Timeout(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x0"}`))
	})

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	n := testClientCounter.Add(1)
	namespace := "test_timeout_" + strconv.FormatInt(n, 10)

	client := NewExecutionClient(log, NewMetrics(namespace), "node-1", server.URL, nil, 50*time.Millisecond)

	_, err := client.ETHGetBalance(context.Background(), "0x1234567890123456789012345678901234567890", "latest")
	assert.Error(t, err)
}

func TestExecutionClient_ConnectionRefused(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, "node-1", "http://127.0.0.1:1")

	_, err := client.ETHGetBalance(context.Background(), "0x1234567890123456789012345678901234567890", "latest")
	assert.Error(t, err)
}
