package kafka

import (
	"fmt"
	"math/rand"
	"os"
)

// SASL config
type SASL struct {
	Enabled   bool   `json:"enabled"`
	Handshake bool   `json:"handshake"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}

// TLS config
type TLS struct {
	Enabled  bool `json:"enabled"`
	Insecure bool `json:"insecure"`
}

// Config is the Kafka configuration
type Config struct {
	Enabled      bool     `json:"enabled"`
	Brokers      []string `json:"brokers"`
	Topic        string   `json:"topic"`
	ClientID     string   `json:"client-id"`
	GroupID      string   `json:"group-id"`
	LogVerbosity string   `json:"log-verbosity"`
	TLS          TLS      `json:"tls"`
	SASL         SASL     `json:"sasl"`
}

// DefaultGroupName returns a automatically generated group name based on the host
func DefaultGroupName() string {
	if hostname, err := os.Hostname(); err == nil {
		return fmt.Sprintf("naiserator-%s", hostname)
	}
	return fmt.Sprintf("naiserator-%d", rand.Int())
}
