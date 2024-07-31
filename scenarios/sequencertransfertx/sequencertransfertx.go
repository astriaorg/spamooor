package sequencertransfertx

import (
	"context"
	"fmt"
	grpc_receiver "github.com/astriaorg/spamooor/protos"
	"math/rand"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/astriaorg/spamooor/scenariotypes"
	"github.com/astriaorg/spamooor/tester"
	"github.com/astriaorg/spamooor/utils"
)

type ScenarioOptions struct {
	TotalCount      uint64
	Throughput      uint64
	MaxWallets      uint64
	ComposerAddress string
}

type Scenario struct {
	options      ScenarioOptions
	logger       *logrus.Entry
	tester       *tester.Tester
	composerConn *grpc.ClientConn
}

func NewScenario() scenariotypes.Scenario {
	return &Scenario{
		logger: logrus.WithField("scenario", "sequencertransfertx"),
	}
}

func (s *Scenario) Flags(flags *pflag.FlagSet) error {
	flags.Uint64VarP(&s.options.TotalCount, "count", "c", 0, "Total number of transfer transactions to send")
	flags.Uint64VarP(&s.options.Throughput, "throughput", "t", 0, "Number of transfer transactions to send per slot")
	flags.Uint64Var(&s.options.MaxWallets, "max-wallets", 0, "Maximum number of child wallets to use")
	flags.StringVar(&s.options.ComposerAddress, "composer-address", "localhost:50051", "Address of the composer service")

	return nil
}

func (s *Scenario) Init(testerCfg *tester.TesterConfig) error {
	if s.options.TotalCount == 0 && s.options.Throughput == 0 {
		return fmt.Errorf("neither total count nor throughput limit set, must define at least one of them")
	}

	if s.options.MaxWallets > 0 {
		testerCfg.WalletCount = s.options.MaxWallets
	} else if s.options.TotalCount > 0 {
		if s.options.TotalCount < 1000 {
			testerCfg.WalletCount = s.options.TotalCount
		} else {
			testerCfg.WalletCount = 1000
		}
	} else {
		if s.options.Throughput*10 < 1000 {
			testerCfg.WalletCount = s.options.Throughput * 10
		} else {
			testerCfg.WalletCount = 1000
		}
	}

	conn, err := grpc.NewClient(s.options.ComposerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	s.composerConn = conn

	return nil
}

func (s *Scenario) Setup(testerCfg *tester.Tester) error {
	s.tester = testerCfg

	return nil
}

func (s *Scenario) Run() error {
	txIdxCounter := uint64(0)
	counterMutex := sync.Mutex{}
	waitGroup := sync.WaitGroup{}
	pendingCount := uint64(0)
	txCount := uint64(0)
	startTime := time.Now()

	s.logger.Infof("starting scenario: sequencertransfertx")

	for {
		txIdx := txIdxCounter
		txIdxCounter++

		waitGroup.Add(1)
		counterMutex.Lock()
		pendingCount++
		counterMutex.Unlock()

		go func(txIdx uint64) {
			defer func() {
				counterMutex.Lock()
				pendingCount--
				counterMutex.Unlock()
				waitGroup.Done()
			}()

			logger := s.logger
			err := s.sendTx()
			if err != nil {
				logger.Warnf("could not send transaction: %v", err)
				return
			}

			counterMutex.Lock()
			txCount++
			counterMutex.Unlock()
			logger.Infof("sent sequencer transfer tx #%6d:", txIdx+1)
		}(txIdx)

		count := txCount + pendingCount
		if s.options.TotalCount > 0 && count >= s.options.TotalCount {
			break
		}
		if s.options.Throughput > 0 {
			for count/((uint64(time.Since(startTime).Seconds())/utils.SecondsPerSlot)+1) >= s.options.Throughput {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
	waitGroup.Wait()

	s.logger.Infof("finished sending transactions, awaiting block inclusion...")

	return nil
}

func (s *Scenario) sendTx() error {

	err := SendSequencerTransferViaComposer(s.composerConn)
	if err != nil {
		return err
	}

	return nil
}

func SendSequencerTransferViaComposer(conn *grpc.ClientConn) error {
	grpcCollectorServiceClient := grpc_receiver.NewSequencerGrpcCollectorServiceClient(conn)

	// get a random value b/w 10000 and 10000000
	amount := uint64(10000 + rand.Intn(10000000-10000))

	_, err := grpcCollectorServiceClient.SubmitSequencerTransaction(context.Background(), &grpc_receiver.SubmitSequencerTransactionRequest{Action: &grpc_receiver.Action{Value: &grpc_receiver.Action_TransferAction{TransferAction: &grpc_receiver.TransferAction{
		To: &grpc_receiver.Address{Bech32M: "astria1mzr3gdek0c5jxlk34uq959x4hpjs3vmxjfj2p7"},
		Amount: &grpc_receiver.Uint128{
			Lo: amount,
			Hi: 0,
		},
		Asset:    "nria",
		FeeAsset: "nria",
	}}}})
	if err != nil {
		return err
	}

	return nil
}
