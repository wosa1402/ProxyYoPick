package source

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/efan/proxyyopick/internal/model"
)

// FileSource reads proxies from a text file (ip:port per line).
type FileSource struct {
	Path string
}

func NewFileSource(path string) *FileSource {
	return &FileSource{Path: path}
}

func (f *FileSource) Name() string {
	return "file:" + f.Path
}

func (f *FileSource) Fetch(_ context.Context) (model.ProxyList, error) {
	file, err := os.Open(f.Path)
	if err != nil {
		return nil, fmt.Errorf("open file %s: %w", f.Path, err)
	}
	defer file.Close()

	var proxies model.ProxyList
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		proxy, ok := parseLine(line)
		if !ok {
			slog.Warn("skipping malformed line", "file", f.Path, "line", lineNum, "content", line)
			continue
		}
		proxies = append(proxies, proxy)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read file %s: %w", f.Path, err)
	}

	slog.Info("loaded proxies from file", "file", f.Path, "count", len(proxies))
	return proxies, nil
}

// parseLine parses a line in "ip:port" or "protocol://ip:port" format.
// Lines with authentication (ip:port:user:pass) are silently skipped.
func parseLine(line string) (model.Proxy, bool) {
	protocol := "socks5"

	// Handle protocol://ip:port format
	if idx := strings.Index(line, "://"); idx != -1 {
		protocol = line[:idx]
		line = line[idx+3:]
	}

	// Skip lines with auth credentials (ip:port:user:pass)
	parts := strings.Split(line, ":")
	if len(parts) > 2 {
		return model.Proxy{}, false
	}
	if len(parts) != 2 {
		return model.Proxy{}, false
	}

	ip := strings.TrimSpace(parts[0])
	portStr := strings.TrimSpace(parts[1])

	if ip == "" || portStr == "" {
		return model.Proxy{}, false
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return model.Proxy{}, false
	}

	return model.Proxy{
		IP:       ip,
		Port:     port,
		Protocol: protocol,
	}, true
}
