package scenariotypes

import (
	"github.com/astriaorg/spamooor/tester"
	"github.com/spf13/pflag"
)

type Scenario interface {
	Flags(flags *pflag.FlagSet) error
	Init(testerCfg *tester.TesterConfig) error
	Setup(testerCfg *tester.Tester) error
	Run() error
}
