package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/efan/proxyyopick/internal/scoring"
	"github.com/spf13/cobra"
)

var (
	concurrency int
	timeout     time.Duration
	targetURL   string
	formats     []string
	outputDir   string

	// IP scoring API keys
	ipqsKey         string
	scamalyticsUser string
	scamalyticsKey  string
	abuseIPDBKey    string
	scoreCachePath  string
)

var rootCmd = &cobra.Command{
	Use:   "proxyyopick",
	Short: "SOCKS5 proxy optimizer and selector",
	Long: `ProxyYoPick - SOCKS5 代理优选工具

从 socks5-proxy.github.io 抓取或从文件导入代理列表，
并发测试连通性和延迟，输出优选结果。`,
}

func init() {
	rootCmd.PersistentFlags().IntVarP(&concurrency, "concurrency", "c", 500, "并发测试数")
	rootCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "t", 10*time.Second, "单个代理超时时间")
	rootCmd.PersistentFlags().StringVarP(&targetURL, "target", "T", "http://www.google.com/generate_204", "测试目标 URL")
	rootCmd.PersistentFlags().StringSliceVarP(&formats, "format", "f", []string{"table"}, "输出格式: table, txt, json")
	rootCmd.PersistentFlags().StringVarP(&outputDir, "output-dir", "o", "./output", "输出目录")

	// IP scoring flags
	rootCmd.PersistentFlags().StringVar(&ipqsKey, "ipqs-key", "", "IPQualityScore API key (或设置 IPQS_KEY 环境变量)")
	rootCmd.PersistentFlags().StringVar(&scamalyticsUser, "scamalytics-user", "", "Scamalytics 用户名 (或设置 SCAMALYTICS_USER 环境变量)")
	rootCmd.PersistentFlags().StringVar(&scamalyticsKey, "scamalytics-key", "", "Scamalytics API key (或设置 SCAMALYTICS_KEY 环境变量)")
	rootCmd.PersistentFlags().StringVar(&abuseIPDBKey, "abuseipdb-key", "", "AbuseIPDB API key (或设置 ABUSEIPDB_KEY 环境变量)")
	rootCmd.PersistentFlags().StringVar(&scoreCachePath, "score-cache", "", "评分缓存文件路径 (默认: ~/.proxyyopick/score_cache.json)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// resolveScoreCfg builds a scoring.Config from flags and environment variables.
func resolveScoreCfg() scoring.Config {
	return scoring.Config{
		IPQSKey:         firstNonEmpty(ipqsKey, os.Getenv("IPQS_KEY")),
		ScamalyticsUser: firstNonEmpty(scamalyticsUser, os.Getenv("SCAMALYTICS_USER")),
		ScamalyticsKey:  firstNonEmpty(scamalyticsKey, os.Getenv("SCAMALYTICS_KEY")),
		AbuseIPDBKey:    firstNonEmpty(abuseIPDBKey, os.Getenv("ABUSEIPDB_KEY")),
		CachePath:       firstNonEmpty(scoreCachePath, os.Getenv("SCORE_CACHE_PATH")),
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
