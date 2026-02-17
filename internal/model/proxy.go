package model

import (
	"fmt"
	"time"
)

// Proxy represents a single SOCKS5 proxy endpoint.
type Proxy struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol,omitempty"`
	Location string `json:"location,omitempty"`
}

// Address returns the "ip:port" string.
func (p Proxy) Address() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

// Key returns a deduplication key.
func (p Proxy) Key() string {
	return p.Address()
}

// TestResult holds the outcome of testing a single proxy.
type TestResult struct {
	Proxy    Proxy         `json:"proxy"`
	Success  bool          `json:"success"`
	Latency  time.Duration `json:"-"`
	LatencyMs int64        `json:"latency_ms"`
	Error    string        `json:"error,omitempty"`
	TestedAt time.Time     `json:"tested_at"`
}

// ProxyList is a collection of proxies.
type ProxyList []Proxy
