package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/efan/proxyyopick/internal/model"
	"github.com/efan/proxyyopick/internal/source"
	"github.com/spf13/cobra"
)

var useStdin bool

var importCmd = &cobra.Command{
	Use:   "import [files...]",
	Short: "从文件或 stdin 导入代理列表，测试并输出结果",
	RunE:  runImport,
}

func init() {
	importCmd.Flags().BoolVar(&useStdin, "stdin", false, "从 stdin 读取代理列表")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	var allProxies model.ProxyList

	// Load from files
	for _, path := range args {
		src := source.NewFileSource(path)
		proxies, err := src.Fetch(ctx)
		if err != nil {
			return fmt.Errorf("读取文件 %s 失败: %w", path, err)
		}
		allProxies = append(allProxies, proxies...)
	}

	// Load from stdin
	if useStdin || len(args) == 0 {
		src := source.NewTextSource(os.Stdin, "stdin")
		proxies, err := src.Fetch(ctx)
		if err != nil {
			return fmt.Errorf("读取 stdin 失败: %w", err)
		}
		allProxies = append(allProxies, proxies...)
	}

	// Deduplicate
	allProxies = source.Deduplicate(allProxies)
	fmt.Printf("📦 获取到 %d 个代理（已去重）\n", len(allProxies))

	if len(allProxies) == 0 {
		fmt.Println("⚠️  未获取到任何代理")
		return nil
	}

	return testAndOutput(ctx, allProxies)
}
