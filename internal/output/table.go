package output

import (
	"fmt"
	"os"
	"strconv"

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
	table.SetHeader([]string{"#", "IP", "Port", "Country", "Quality", "Latency(ms)", "Status"})
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
			latencyStr,
			status,
		})
	}

	fmt.Printf("\n📊 Results: %d/%d successful\n\n", successCount, len(results))
	table.Render()
	fmt.Println()

	return nil
}
