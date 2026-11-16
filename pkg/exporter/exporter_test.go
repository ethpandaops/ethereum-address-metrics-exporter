package exporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "no credentials", url: "http://localhost:8545", want: "http://localhost:8545"},
		{name: "username and password", url: "https://user:testpass@rpc.example.com/path", want: "https://user:xxxxx@rpc.example.com/path"}, //nolint:gosec // placeholder test credentials
		{name: "unparseable", url: "https://user:testpass%zz@rpc.example.com", want: "<invalid URL>"},                                       //nolint:gosec // placeholder test credentials
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, redactURL(tt.url))
		})
	}
}
