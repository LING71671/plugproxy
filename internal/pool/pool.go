package pool

import "github.com/LING71671/plugproxy/pkg/model"

type Strategy string

const (
	StrategyAny               Strategy = "any"
	StrategyFastest           Strategy = "fastest"
	StrategyRandom            Strategy = "random"
	StrategyRoundRobin        Strategy = "round_robin"
	StrategyLeastRecentlyUsed Strategy = "least_recently_used"
	StrategyWeighted          Strategy = "weighted"
)

type Filter struct {
	Protocol    model.Protocol
	Healthy     bool
	Status      model.HealthStatus
	Source      string
	ExcludeDead bool
}

type Pool interface {
	Add(proxy model.Proxy)
	Get(strategy Strategy, filter Filter) (model.Proxy, bool)
	List(filter Filter) []model.Proxy
}
