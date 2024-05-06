package scenarios

import (
	"github.com/ethpandaops/goomy-blob/scenarios/deploytx"
	"github.com/ethpandaops/goomy-blob/scenarios/eoatx"
	"github.com/ethpandaops/goomy-blob/scenarios/erctx"
	"github.com/ethpandaops/goomy-blob/scenarios/gasburnertx"
	"github.com/ethpandaops/goomy-blob/scenarios/univ2tx"
	"github.com/ethpandaops/goomy-blob/scenariotypes"
)

var Scenarios = map[string]func() scenariotypes.Scenario{
	"eoatx":       eoatx.NewScenario,
	"erctx":       erctx.NewScenario,
	"gasburnertx": gasburnertx.NewScenario,
	"univ2tx":     univ2tx.NewScenario,
	"deploytx":    deploytx.NewScenario,
}
