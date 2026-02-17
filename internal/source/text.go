package source

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"strings"

	"github.com/efan/proxyyopick/internal/model"
)

// TextSource reads proxies from an io.Reader (stdin or raw text).
type TextSource struct {
	Reader io.Reader
	Label  string
}

func NewTextSource(r io.Reader, label string) *TextSource {
	return &TextSource{Reader: r, Label: label}
}

func (t *TextSource) Name() string {
	return "text:" + t.Label
}

func (t *TextSource) Fetch(_ context.Context) (model.ProxyList, error) {
	var proxies model.ProxyList
	scanner := bufio.NewScanner(t.Reader)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		proxy, ok := parseLine(line)
		if !ok {
			slog.Warn("skipping malformed line", "source", t.Label, "line", lineNum, "content", line)
			continue
		}
		proxies = append(proxies, proxy)
	}
	if err := scanner.Err(); err != nil {
		return proxies, err
	}

	slog.Info("loaded proxies from text", "source", t.Label, "count", len(proxies))
	return proxies, nil
}
