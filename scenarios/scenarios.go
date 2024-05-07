package scenarios

import (
	"github.com/astriaorg/spamooor/scenarios/deploytx"
	"github.com/astriaorg/spamooor/scenarios/eoatx"
	"github.com/astriaorg/spamooor/scenarios/erctx"
	"github.com/astriaorg/spamooor/scenarios/gasburnertx"
	"github.com/astriaorg/spamooor/scenarios/univ2tx"
	"github.com/astriaorg/spamooor/scenariotypes"
)

var Scenarios = map[string]func() scenariotypes.Scenario{
	"eoatx":       eoatx.NewScenario,
	"erctx":       erctx.NewScenario,
	"gasburnertx": gasburnertx.NewScenario,
	"univ2tx":     univ2tx.NewScenario,
	"deploytx":    deploytx.NewScenario,
}
