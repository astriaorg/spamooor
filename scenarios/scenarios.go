package scenarios

import (
	"github.com/astriaorg/spamooor/scenarios/deploytx"
	"github.com/astriaorg/spamooor/scenarios/eoatx"
	"github.com/astriaorg/spamooor/scenarios/erctx"
	"github.com/astriaorg/spamooor/scenarios/gasburnertx"
	"github.com/astriaorg/spamooor/scenarios/revertingtx"
	"github.com/astriaorg/spamooor/scenarios/sequencersequenceactiontx"
	"github.com/astriaorg/spamooor/scenarios/sequencertransfertx"
	"github.com/astriaorg/spamooor/scenarios/univ2tx"
	"github.com/astriaorg/spamooor/scenariotypes"
)

var Scenarios = map[string]func() scenariotypes.Scenario{
	"revertingtx":               revertingtx.NewScenario,
	"eoatx":                     eoatx.NewScenario,
	"erctx":                     erctx.NewScenario,
	"gasburnertx":               gasburnertx.NewScenario,
	"univ2tx":                   univ2tx.NewScenario,
	"deploytx":                  deploytx.NewScenario,
	"sequencertransfertx":       sequencertransfertx.NewScenario,
	"sequencersequenceactiontx": sequencersequenceactiontx.NewScenario,
}
