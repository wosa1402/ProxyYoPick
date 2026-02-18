package model

import (
	"fmt"
	"time"
)

// Proxy represents a single SOCKS5 proxy endpoint.
type Proxy struct {
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol,omitempty"`
	Location    string `json:"location,omitempty"`
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	// IP quality fields (from ip-api.com)
	IsProxy   bool   `json:"is_proxy"`
	IsHosting bool   `json:"is_hosting"`
	IsMobile  bool   `json:"is_mobile"`
	ISP       string `json:"isp,omitempty"`
	Quality   string `json:"quality,omitempty"` // "residential", "mobile", "datacenter", "proxy"
	// External IP scoring (from IPQualityScore, Scamalytics, AbuseIPDB)
	Scores IPScores `json:"scores,omitempty"`
}

// IPScores holds fraud/abuse scores from external IP scoring APIs.
// nil values indicate the API was not queried (key not configured).
type IPScores struct {
	IPQS        *int `json:"ipqs,omitempty"`        // IPQualityScore fraud_score (0-100)
	Scamalytics *int `json:"scamalytics,omitempty"` // Scamalytics score (0-100)
	AbuseIPDB   *int `json:"abuseipdb,omitempty"`   // AbuseIPDB abuseConfidenceScore (0-100)
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
