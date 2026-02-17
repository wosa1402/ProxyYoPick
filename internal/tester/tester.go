package tester

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/efan/proxyyopick/internal/model"
	"golang.org/x/net/proxy"
)

// Tester tests a single proxy and returns the result.
type Tester interface {
	Test(ctx context.Context, p model.Proxy) model.TestResult
}

// SOCKS5Tester tests proxy connectivity by making an HTTP request through SOCKS5.
type SOCKS5Tester struct {
	TargetURL string
	Timeout   time.Duration
}

func NewSOCKS5Tester(targetURL string, timeout time.Duration) *SOCKS5Tester {
	return &SOCKS5Tester{
		TargetURL: targetURL,
		Timeout:   timeout,
	}
}

func (t *SOCKS5Tester) Test(ctx context.Context, p model.Proxy) model.TestResult {
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()

	start := time.Now()

	dialer, err := proxy.SOCKS5("tcp", p.Address(), nil, proxy.Direct)
	if err != nil {
		return model.TestResult{
			Proxy:    p,
			Success:  false,
			Error:    err.Error(),
			TestedAt: start,
		}
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   t.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.TargetURL, nil)
	if err != nil {
		return model.TestResult{
			Proxy:    p,
			Success:  false,
			Latency:  time.Since(start),
			Error:    err.Error(),
			TestedAt: start,
		}
	}

	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return model.TestResult{
			Proxy:     p,
			Success:   false,
			Latency:   latency,
			LatencyMs: latency.Milliseconds(),
			Error:     err.Error(),
			TestedAt:  start,
		}
	}
	defer resp.Body.Close()

	return model.TestResult{
		Proxy:     p,
		Success:   true,
		Latency:   latency,
		LatencyMs: latency.Milliseconds(),
		TestedAt:  start,
	}
}
