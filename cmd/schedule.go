package cmd

import (
	"context"
	"time"

	"github.com/efan/proxyyopick/internal/scheduler"
	"github.com/spf13/cobra"
)

var interval time.Duration

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "定时运行代理测试（守护模式）",
	RunE:  runSchedule,
}

func init() {
	scheduleCmd.Flags().DurationVar(&interval, "interval", 30*time.Minute, "执行间隔")
	rootCmd.AddCommand(scheduleCmd)
}

func runSchedule(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	s := scheduler.New(interval)

	return s.Start(ctx, func() error {
		return runRun(cmd, args)
	})
}
