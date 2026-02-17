package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	concurrency int
	timeout     time.Duration
	targetURL   string
	formats     []string
	outputDir   string
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
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
