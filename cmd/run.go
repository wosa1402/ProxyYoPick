package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/efan/proxyyopick/internal/geo"
	"github.com/efan/proxyyopick/internal/model"
	"github.com/efan/proxyyopick/internal/output"
	"github.com/efan/proxyyopick/internal/source"
	"github.com/efan/proxyyopick/internal/tester"
	"github.com/spf13/cobra"
)

var scrapeURL string

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "抓取默认源的代理，测试并输出结果",
	RunE:  runRun,
}

func init() {
	runCmd.Flags().StringVar(&scrapeURL, "url", "https://socks5-proxy.github.io/", "抓取代理的 URL")
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// 1. Fetch proxies
	fmt.Printf("🔍 正在从 %s 抓取代理列表...\n", scrapeURL)
	scraper := source.NewScraper(scrapeURL)
	proxies, err := scraper.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("抓取失败: %w", err)
	}

	// 2. Deduplicate
	proxies = source.Deduplicate(proxies)
	fmt.Printf("📦 获取到 %d 个代理（已去重）\n", len(proxies))

	if len(proxies) == 0 {
		fmt.Println("⚠️  未获取到任何代理")
		return nil
	}

	return testAndOutput(ctx, proxies)
}

// testAndOutput is shared logic for testing proxies and writing output.
func testAndOutput(ctx context.Context, proxies model.ProxyList) error {
	// Test proxies
	fmt.Printf("🚀 开始测试，并发数: %d，超时: %s，目标: %s\n", concurrency, timeout, targetURL)
	t := tester.NewSOCKS5Tester(targetURL, timeout)
	results := tester.RunPool(ctx, t, proxies, tester.PoolConfig{Workers: concurrency}, func(done, total int) {
		pct := float64(done) / float64(total) * 100
		fmt.Printf("\r⏳ 测试进度: %d/%d (%.1f%%)", done, total, pct)
	})

	// Sort results
	results = tester.SortByLatency(results)

	// Lookup countries for tested proxies
	proxyPtrs := make(model.ProxyList, len(results))
	for i := range results {
		proxyPtrs[i] = results[i].Proxy
	}
	geo.LookupCountries(ctx, proxyPtrs)
	for i := range results {
		results[i].Proxy = proxyPtrs[i]
	}

	// Output
	return writeResults(results)
}

func writeResults(results []model.TestResult) error {
	writers := buildWriters()
	for _, w := range writers {
		if err := w.Write(results); err != nil {
			slog.Error("output failed", "error", err)
		}
	}
	return nil
}

func buildWriters() []output.Writer {
	var writers []output.Writer
	for _, f := range formats {
		switch f {
		case "table":
			writers = append(writers, output.NewTableWriter())
		case "txt":
			writers = append(writers, output.NewTxtWriter(outputDir))
		case "json":
			writers = append(writers, output.NewJSONWriter(outputDir))
		}
	}
	return writers
}
