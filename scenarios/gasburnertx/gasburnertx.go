package gasburnertx

import (
	"context"
	"fmt"
	largetx "github.com/astriaorg/spamooor/scenarios/gasburnertx/contracts"
	"github.com/astriaorg/spamooor/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/astriaorg/spamooor/scenariotypes"
	"github.com/astriaorg/spamooor/tester"
	"github.com/astriaorg/spamooor/txbuilder"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

type ScenarioOptions struct {
	TotalCount      uint64
	Throughput      uint64
	MaxPending      uint64
	MaxWallets      uint64
	Timeout         uint64
	BaseFee         uint64
	TipFee          uint64
	GasUnitsToBurn  uint64
	ComposerAddress string
	SendViaComposer bool
	RollupId        string
}

type Scenario struct {
	options      ScenarioOptions
	logger       *logrus.Entry
	tester       *tester.Tester
	composerConn *grpc.ClientConn

	gasBurnerContractAddr common.Address

	pendingCount  uint64
	pendingChan   chan bool
	pendingWGroup sync.WaitGroup
}

func NewScenario() scenariotypes.Scenario {
	return &Scenario{
		logger: logrus.WithField("scenario", "gasburnertx"),
	}
}

func (s *Scenario) Flags(flags *pflag.FlagSet) error {
	flags.Uint64VarP(&s.options.TotalCount, "count", "c", 0, "Total number of large transactions to send")
	flags.Uint64VarP(&s.options.Throughput, "throughput", "t", 0, "Number of large transactions to send per slot")
	flags.Uint64Var(&s.options.MaxPending, "max-pending", 0, "Maximum number of pending transactions")
	flags.Uint64Var(&s.options.MaxWallets, "max-wallets", 0, "Maximum number of child wallets to use")
	flags.Uint64Var(&s.options.Timeout, "timeout", 120, "Number of seconds to wait timing out the test")
	flags.Uint64Var(&s.options.BaseFee, "basefee", 20, "Max fee per gas to use in large transactions (in gwei)")
	flags.Uint64Var(&s.options.TipFee, "tipfee", 2, "Max tip per gas to use in large transactions (in gwei)")
	flags.Uint64Var(&s.options.GasUnitsToBurn, "gas-units-to-burn", 2000000, "The number of gas units for each tx to cost")
	flags.StringVar(&s.options.ComposerAddress, "composer-address", "localhost:50051", "Address of the composer service")
	flags.BoolVar(&s.options.SendViaComposer, "send-via-composer", false, "Send transactions via composer")
	flags.StringVar(&s.options.RollupId, "", "", "The rollup id of the evm rollup")

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

	if s.options.MaxPending > 0 {
		s.pendingChan = make(chan bool, s.options.MaxPending)
	}

	if s.options.SendViaComposer {
		conn, err := grpc.NewClient(s.options.ComposerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return err
		}

		s.composerConn = conn
	}

	return nil
}

func (s *Scenario) Setup(testerCfg *tester.Tester) error {
	s.tester = testerCfg
	s.logger.Infof("setting up scenario: gasburnertx")
	s.logger.Infof("deploying gas burner contract...")
	receipt, _, err := s.DeployGasBurnerContract()
	if err != nil {
		return err
	}

	s.gasBurnerContractAddr = receipt.ContractAddress

	s.logger.Infof("deployed gas burner contract at %v", s.gasBurnerContractAddr.String())

	return nil
}

func (s *Scenario) Run() error {
	txIdxCounter := uint64(0)
	counterMutex := sync.Mutex{}
	waitGroup := sync.WaitGroup{}
	pendingCount := uint64(0)
	txCount := uint64(0)
	startTime := time.Now()

	s.logger.Infof("starting scenario: gasburnertx")

	for {
		txIdx := txIdxCounter
		txIdxCounter++

		if s.pendingChan != nil {
			// await pending transactions
			s.pendingChan <- true
		}
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
			tx, client, err := s.sendTx(txIdx)
			if client != nil {
				logger = logger.WithField("rpc", client.GetName())
			}
			if err != nil {
				logger.Warnf("could not send transaction: %v", err)
				<-s.pendingChan
				return
			}

			counterMutex.Lock()
			txCount++
			counterMutex.Unlock()
			logger.Infof("sent tx #%6d: %v", txIdx+1, tx.Hash().String())
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
	s.pendingWGroup.Wait()
	s.logger.Infof("finished sending transactions, awaiting block inclusion...")

	return nil
}

func (s *Scenario) DeployGasBurnerContract() (*types.Receipt, *txbuilder.Client, error) {
	wallet := s.tester.GetRootWallet()
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	transactor, err := s.GetTransactor(wallet, true, big.NewInt(0))
	if err != nil {
		return nil, nil, err
	}

	_, deployTx, _, err := largetx.DeployGasBurner(transactor, client.GetEthClient())
	if err != nil {
		return nil, nil, err
	}

	receipt, _, err := txbuilder.SendAndAwaitTx(txbuilder.SendTxOpts{
		Wallet:  wallet,
		Tx:      deployTx,
		Client:  client,
		BaseFee: int64(s.options.BaseFee),
		TipFee:  int64(s.options.TipFee),
		Gas:     2000000,
	})
	if err != nil {
		return nil, nil, err
	}

	return receipt, client, nil
}

func (s *Scenario) sendTx(txIdx uint64) (*types.Transaction, *txbuilder.Client, error) {
	client := s.tester.GetClient(tester.SelectByIndex, int(txIdx))
	wallet := s.tester.GetWallet(tester.SelectByIndex, int(txIdx))

	var feeCap *big.Int
	var tipCap *big.Int

	if s.options.BaseFee > 0 {
		feeCap = new(big.Int).Mul(big.NewInt(int64(s.options.BaseFee)), big.NewInt(1000000000))
	}
	if s.options.TipFee > 0 {
		tipCap = new(big.Int).Mul(big.NewInt(int64(s.options.TipFee)), big.NewInt(1000000000))
	}

	if feeCap == nil || tipCap == nil {
		var err error
		feeCap, tipCap, err = client.GetSuggestedFee()
		if err != nil {
			return nil, client, err
		}
	}

	if feeCap.Cmp(big.NewInt(1000000000)) < 0 {
		feeCap = big.NewInt(1000000000)
	}
	if tipCap.Cmp(big.NewInt(1000000000)) < 0 {
		tipCap = big.NewInt(1000000000)
	}

	gasBurnerContract, err := s.GetGasBurner()
	if err != nil {
		return nil, nil, err
	}

	transactor, err := s.GetTransactor(wallet, true, big.NewInt(0))
	if err != nil {
		return nil, nil, err
	}

	s.logger.Infof("gas units to burn for tx %d: %d", txIdx+1, s.options.GasUnitsToBurn)
	gasBurnerTx, err := gasBurnerContract.BurnGasUnits(transactor, big.NewInt(int64(s.options.GasUnitsToBurn)))
	if err != nil {
		s.logger.Errorf("could not generate transaction: %v", err)
		return nil, nil, err
	}

	txData, err := txbuilder.DynFeeTx(&txbuilder.TxMetadata{
		GasFeeCap: uint256.MustFromBig(feeCap),
		GasTipCap: uint256.MustFromBig(tipCap),
		Gas:       gasBurnerTx.Gas(),
		To:        &s.gasBurnerContractAddr,
		Value:     uint256.NewInt(0),
		Data:      gasBurnerTx.Data(),
	})
	if err != nil {
		return nil, nil, err
	}

	tx, err := wallet.BuildDynamicFeeTx(txData)
	if err != nil {
		return nil, nil, err
	}

	if s.options.SendViaComposer {
		err = client.SendTransactionViaComposer(tx, s.composerConn, s.options.RollupId)
		if err != nil {
			return nil, client, err
		}
	} else {
		err = client.SendTransaction(tx)
		if err != nil {
			return nil, client, err
		}
	}

	s.pendingWGroup.Add(1)
	go s.awaitTx(txIdx, tx, client, wallet)

	return tx, client, nil
}

func (s *Scenario) awaitTx(txIdx uint64, tx *types.Transaction, client *txbuilder.Client, wallet *txbuilder.Wallet) {
	var awaitConfirmation = true
	defer func() {
		awaitConfirmation = false
		if s.pendingChan != nil {
			<-s.pendingChan
		}
		s.pendingWGroup.Done()
	}()
	if s.options.Timeout > 0 {
		go s.timeTicker(txIdx, tx, &awaitConfirmation)
	}

	receipt, blockNum, err := client.AwaitTransaction(tx)
	if err != nil {
		s.logger.WithField("client", client.GetName()).Warnf("error while awaiting tx receipt: %v", err)
		return
	}

	effectiveGasPrice := receipt.EffectiveGasPrice
	if effectiveGasPrice == nil {
		effectiveGasPrice = big.NewInt(0)
	}
	blobGasPrice := receipt.BlobGasPrice
	if blobGasPrice == nil {
		blobGasPrice = big.NewInt(0)
	}
	feeAmount := new(big.Int).Mul(effectiveGasPrice, big.NewInt(int64(receipt.GasUsed)))
	totalAmount := new(big.Int).Add(tx.Value(), feeAmount)
	wallet.SubBalance(totalAmount)

	gweiTotalFee := new(big.Int).Div(totalAmount, big.NewInt(1000000000))
	gweiBaseFee := new(big.Int).Div(effectiveGasPrice, big.NewInt(1000000000))
	gweiBlobFee := new(big.Int).Div(blobGasPrice, big.NewInt(1000000000))

	txStatus := "failure"
	if receipt.Status == 1 {
		txStatus = "success"
	}

	s.logger.WithField("client", client.GetName()).Infof(" transaction %d confirmed in block #%v with %s. total gas units: %d, total fee: %v gwei (base: %v, blob: %v)", txIdx+1, blockNum, txStatus, receipt.GasUsed, gweiTotalFee, gweiBaseFee, gweiBlobFee)
}

func (s *Scenario) timeTicker(txIdx uint64, tx *types.Transaction, awaitConfirmation *bool) {
	for {
		time.Sleep(time.Duration(s.options.Timeout) * time.Second)

		if !*awaitConfirmation {
			break
		}

		s.logger.Infof("timeout reached for tx: %d with hash: %s, stopping test", txIdx, tx.Hash().String())
		os.Exit(1)
	}
}

func (s *Scenario) GetTransactor(wallet *txbuilder.Wallet, noSend bool, value *big.Int) (*bind.TransactOpts, error) {
	transactor, err := bind.NewKeyedTransactorWithChainID(wallet.GetPrivateKey(), wallet.GetChainId())
	if err != nil {
		return nil, err
	}
	transactor.Context = context.Background()
	transactor.NoSend = noSend
	transactor.Value = value

	return transactor, nil
}

func (s *Scenario) GetGasBurner() (*largetx.GasBurner, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)
	return largetx.NewGasBurner(s.gasBurnerContractAddr, client.GetEthClient())
}
