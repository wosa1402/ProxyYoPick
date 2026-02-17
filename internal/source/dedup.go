package source

import "github.com/efan/proxyyopick/internal/model"

// Deduplicate removes duplicate proxies, preserving first occurrence order.
func Deduplicate(proxies model.ProxyList) model.ProxyList {
	seen := make(map[string]struct{}, len(proxies))
	result := make(model.ProxyList, 0, len(proxies))
	for _, p := range proxies {
		key := p.Key()
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			result = append(result, p)
		}
	}
	return result
}
