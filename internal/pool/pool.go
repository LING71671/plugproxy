package pool

import "github.com/LING71671/plugproxy/pkg/model"

type Strategy string

const (
	StrategyAny     Strategy = "any"
	StrategyFastest Strategy = "fastest"
)

type Filter struct {
	Protocol model.Protocol
	Healthy  bool
	Status   model.HealthStatus
	Source   string
}

type Pool interface {
	Add(proxy model.Proxy)
	Get(strategy Strategy, filter Filter) (model.Proxy, bool)
	List(filter Filter) []model.Proxy
}
