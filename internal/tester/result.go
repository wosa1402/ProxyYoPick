package tester

import (
	"sort"

	"github.com/efan/proxyyopick/internal/model"
)

// SortByLatency sorts results: successful proxies first (ascending latency), then failed.
func SortByLatency(results []model.TestResult) []model.TestResult {
	sorted := make([]model.TestResult, len(results))
	copy(sorted, results)

	sort.SliceStable(sorted, func(i, j int) bool {
		// Successful proxies come first
		if sorted[i].Success != sorted[j].Success {
			return sorted[i].Success
		}
		// Among successful, sort by latency ascending
		if sorted[i].Success && sorted[j].Success {
			return sorted[i].Latency < sorted[j].Latency
		}
		return false
	})

	return sorted
}

// FilterSuccessful returns only successful test results.
func FilterSuccessful(results []model.TestResult) []model.TestResult {
	var out []model.TestResult
	for _, r := range results {
		if r.Success {
			out = append(out, r)
		}
	}
	return out
}
