package cmd

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/efan/proxyyopick/internal/web"
	"github.com/spf13/cobra"
)

var (
	webAddr     string
	webInterval time.Duration
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "启动 Web 仪表盘（含定时优选）",
	RunE:  runWeb,
}

func init() {
	webCmd.Flags().StringVar(&webAddr, "addr", ":8080", "Web 服务监听地址")
	webCmd.Flags().DurationVar(&webInterval, "interval", 30*time.Minute, "定时优选间隔")
	webCmd.Flags().StringVar(&scrapeURL, "url", "https://socks5-proxy.github.io/", "抓取代理的 URL")
	rootCmd.AddCommand(webCmd)
}

func runWeb(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	srv := web.NewServer(web.Config{
		Addr:        webAddr,
		ScrapeURL:   scrapeURL,
		Concurrency: concurrency,
		Timeout:     timeout,
		TargetURL:   targetURL,
		Interval:    webInterval,
		ScoreCfg:    resolveScoreCfg(),
	})

	return srv.Start(ctx)
}
