package source

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/efan/proxyyopick/internal/model"
	"golang.org/x/net/html"
)

// Scraper fetches proxies from the socks5-proxy.github.io HTML page.
type Scraper struct {
	URL string
}

func NewScraper(url string) *Scraper {
	return &Scraper{URL: url}
}

func (s *Scraper) Name() string {
	return "scraper:" + s.URL
}

func (s *Scraper) Fetch(ctx context.Context) (model.ProxyList, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	var proxies model.ProxyList
	// Walk the DOM to find table rows
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			proxy, ok := parseTableRow(n)
			if ok {
				proxies = append(proxies, proxy)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	slog.Info("scraped proxies", "source", s.URL, "count", len(proxies))
	return proxies, nil
}

// parseTableRow extracts a proxy from a <tr> element by looking at <td> class attributes.
func parseTableRow(tr *html.Node) (model.Proxy, bool) {
	var ip, portStr, protocol, location string

	for td := tr.FirstChild; td != nil; td = td.NextSibling {
		if td.Type != html.ElementNode || td.Data != "td" {
			continue
		}
		class := getAttr(td, "class")
		text := strings.TrimSpace(getTextContent(td))
		switch class {
		case "ip":
			ip = text
		case "port":
			portStr = text
		case "protocol":
			protocol = text
		case "location":
			location = text
		}
	}

	if ip == "" || portStr == "" {
		return model.Proxy{}, false
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return model.Proxy{}, false
	}

	if protocol == "" {
		protocol = "socks5"
	}

	return model.Proxy{
		IP:       ip,
		Port:     port,
		Protocol: protocol,
		Location: location,
	}, true
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func getTextContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(getTextContent(c))
	}
	return sb.String()
}
