package output

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/efan/proxyyopick/internal/model"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

// TableWriter writes results as a colored terminal table.
type TableWriter struct{}

func NewTableWriter() *TableWriter {
	return &TableWriter{}
}

func (w *TableWriter) Write(results []model.TestResult) error {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"#", "IP", "Port", "Country", "Quality", "Scores", "Apple", "Latency(ms)", "Status"})
	table.SetBorder(true)
	table.SetAutoWrapText(false)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	cyan := color.New(color.FgCyan).SprintFunc()
	successCount := 0
	for i, r := range results {
		var status, latencyStr string

		if r.Success {
			successCount++
			ms := r.LatencyMs
			latencyStr = strconv.FormatInt(ms, 10)
			if ms < 1000 {
				status = green("OK")
				latencyStr = green(latencyStr)
			} else {
				status = yellow("SLOW")
				latencyStr = yellow(latencyStr)
			}
		} else {
			status = red("FAIL")
			latencyStr = "-"
			if r.Error != "" {
				errMsg := r.Error
				if len(errMsg) > 40 {
					errMsg = errMsg[:40] + "..."
				}
				status = red("FAIL: " + errMsg)
			}
		}

		country := r.Proxy.Country
		if r.Proxy.CountryCode != "" {
			country = r.Proxy.CountryCode
		}

		var qualityStr string
		switch r.Proxy.Quality {
		case "residential":
			qualityStr = green("residential")
		case "mobile":
			qualityStr = cyan("mobile")
		case "datacenter":
			qualityStr = yellow("datacenter")
		case "proxy":
			qualityStr = red("proxy")
		default:
			qualityStr = "-"
		}

		table.Append([]string{
			strconv.Itoa(i + 1),
			r.Proxy.IP,
			strconv.Itoa(r.Proxy.Port),
			country,
			qualityStr,
			formatScores(r.Proxy.Scores, green, yellow, red),
			formatApple(r.Proxy.AppleBanned, green, red),
			latencyStr,
			status,
		})
	}

	fmt.Printf("\n📊 Results: %d/%d successful\n\n", successCount, len(results))
	table.Render()
	fmt.Println()

	return nil
}

// formatScores formats IPScores as "IPQS/Scam/Abuse" with color coding.
func formatScores(s model.IPScores, green, yellow, red func(a ...interface{}) string) string {
	if s.IPQS == nil && s.Scamalytics == nil && s.AbuseIPDB == nil {
		return "-"
	}
	parts := []string{
		colorScore(s.IPQS, green, yellow, red),
		colorScore(s.Scamalytics, green, yellow, red),
		colorScore(s.AbuseIPDB, green, yellow, red),
	}
	return strings.Join(parts, "/")
}

func colorScore(v *int, green, yellow, red func(a ...interface{}) string) string {
	if v == nil {
		return "-"
	}
	s := strconv.Itoa(*v)
	switch {
	case *v <= 30:
		return green(s)
	case *v <= 60:
		return yellow(s)
	default:
		return red(s)
	}
}

func formatApple(banned *bool, green, red func(a ...interface{}) string) string {
	if banned == nil {
		return "-"
	}
	if *banned {
		return red("封禁")
	}
	return green("正常")
}
