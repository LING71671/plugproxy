package checker

import (
	"context"
	"time"

	"github.com/LING71671/plugproxy/internal/errtype"
	"github.com/LING71671/plugproxy/pkg/model"
)

type Result struct {
	Proxy       model.Proxy
	OK          bool
	Unsupported bool
	Latency     time.Duration
	Error       error
	ErrorType   errtype.Type
}

type Checker interface {
	Check(ctx context.Context, proxy model.Proxy) Result
}
