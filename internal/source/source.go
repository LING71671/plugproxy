package source

import (
	"context"

	"github.com/LING71671/plugproxy/pkg/model"
)

type Source interface {
	Name() string
	Fetch(ctx context.Context) ([]model.Proxy, error)
}

type URLProvider interface {
	SourceURL() string
}
