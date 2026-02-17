package source

import (
	"context"

	"github.com/efan/proxyyopick/internal/model"
)

// Source is the interface for any proxy data provider.
type Source interface {
	Name() string
	Fetch(ctx context.Context) (model.ProxyList, error)
}
