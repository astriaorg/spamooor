package univ3swaptx

import (
	rand2 "crypto/rand"
	"flag"
	"fmt"
	univ2tx "github.com/astriaorg/spamooor/scenarios/univ2tx/contracts"
	univ3swaptx "github.com/astriaorg/spamooor/scenarios/univ3swaptx/contracts"
	"github.com/astriaorg/spamooor/scenariotypes"
	"github.com/astriaorg/spamooor/tester"
	"github.com/astriaorg/spamooor/txbuilder"
	"github.com/astriaorg/spamooor/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"math/big"
	"os"
	"sync"
	"time"
)

type ScenarioOptions struct {
	TotalCount                       uint64
	Throughput                       uint64
	MaxPending                       uint64
	MaxWallets                       uint64
	Timeout                          uint64
	BaseFee                          uint64
	TipFee                           uint64
	WethContractAddress              string
	SwapRouterContractAddress        string
	CustomizableErc20ContractAddress string
	AmountToSwap                     uint64
	RandomAmountToSwap               bool
	TokenMintAmount                  uint64
	ComposerAddress                  string
	SendViaComposer                  bool
	RollupId                         string
}

type Scenario struct {
	options      ScenarioOptions
	logger       *logrus.Entry
	tester       *tester.Tester
	composerConn *grpc.ClientConn

	wethContract              common.Address
	swapRouterContract        common.Address
	customizableErc20Contract common.Address

	tokenMintAmount *big.Int

	pendingCount  uint64
	pendingChan   chan bool
	pendingWGroup sync.WaitGroup
}

func NewScenario() scenariotypes.Scenario {
	return &Scenario{
		logger: logrus.WithField("scenario", "univ3swaptx"),
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
	flag.Uint64Var(&s.options.AmountToSwap, "amount-to-swap", 1, "Amount of tokens to swap in each transaction(in gwei)")
	flag.BoolVar(&s.options.RandomAmountToSwap, "random-amount-to-swap", false, "Randomize the amount of tokens to swap in each transaction(in gwei)")
	flags.StringVar(&s.options.ComposerAddress, "composer-address", "localhost:50051", "Address of the composer service")
	flags.BoolVar(&s.options.SendViaComposer, "send-via-composer", false, "Send transactions via composer")
	flags.StringVar(&s.options.WethContractAddress, "weth-contract", "", "The address of the WETH contract")
	flags.StringVar(&s.options.SwapRouterContractAddress, "swap-router-contract", "", "The address of the Uniswap V2 Router contract")
	flags.StringVar(&s.options.CustomizableErc20ContractAddress, "customizable-erc20-contract", "", "The address of the customizable ERC20 contract")
	flags.Uint64Var(&s.options.TokenMintAmount, "token-mint-amount", 1000000000000000000, "Amount of tokens to mint for each wallet(in gwei)")
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

	s.wethContract = common.HexToAddress(s.options.WethContractAddress)
	s.swapRouterContract = common.HexToAddress(s.options.SwapRouterContractAddress)
	s.customizableErc20Contract = common.HexToAddress(s.options.CustomizableErc20ContractAddress)

	s.tokenMintAmount = big.NewInt(int64(s.options.TokenMintAmount))

	conn, err := grpc.NewClient(s.options.ComposerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	s.composerConn = conn

	return nil
}

func (s *Scenario) Setup(testerCfg *tester.Tester) error {
	s.tester = testerCfg

	s.logger.Infof("starting scenario: univ3tx")

	// now we need to mint DAI and WETH for all child wallets
	s.logger.Infof("minting Erc20 and WETH for child wallets...")
	errorMap, err := s.MintErc20AndWethForChildWallets()
	if err != nil {
		s.logger.Errorf("could not mint Erc20 and WETH for child wallets: %v", err)
		return err
	}
	if len(errorMap) > 0 {
		// print errors
		for addr, errs := range errorMap {
			for _, e := range errs {
				s.logger.Errorf("error for wallet: %v: %v", addr.String(), e)
			}
		}
	}

	return nil
}

func (s *Scenario) Run() error {
	txIdxCounter := uint64(0)
	counterMutex := sync.Mutex{}
	waitGroup := sync.WaitGroup{}
	pendingCount := uint64(0)
	txCount := uint64(0)
	startTime := time.Now()

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

	routerContract, err := s.GetSwapRouterContract()
	if err != nil {
		return nil, nil, err
	}

	// Generate a random number (either 0 or 1)
	result, err := rand2.Int(rand2.Reader, big.NewInt(2))
	if err != nil {
		return nil, nil, err
	}
	var swapDirection []common.Address
	if result.Uint64() == 0 {
		swapDirection = []common.Address{s.customizableErc20Contract, s.wethContract}
	} else {
		swapDirection = []common.Address{s.wethContract, s.customizableErc20Contract}
	}

	// get amount to swap
	amount := uint256.NewInt(s.options.AmountToSwap)
	amount = amount.Mul(amount, uint256.NewInt(1000000))
	if s.options.RandomAmountToSwap {
		n, err := rand2.Int(rand2.Reader, amount.ToBig())
		if err == nil {
			amount = uint256.MustFromBig(n)
		}
	}

	walletTransactor, err := wallet.GetTransactor(true, big.NewInt(0))
	if err != nil {
		return nil, nil, err
	}

	swapTx, err := routerContract.ExactInputSingle(walletTransactor, univ3swaptx.IV3SwapRouterExactInputSingleParams{
		TokenIn:           swapDirection[0],
		TokenOut:          swapDirection[1],
		Fee:               big.NewInt(0),
		Recipient:         walletTransactor.From,
		AmountIn:          amount.ToBig(),
		AmountOutMinimum:  big.NewInt(1),
		SqrtPriceLimitX96: big.NewInt(0),
	})
	if err != nil {
		return nil, nil, err
	}

	txData, err := txbuilder.DynFeeTx(&txbuilder.TxMetadata{
		GasFeeCap: uint256.MustFromBig(feeCap),
		GasTipCap: uint256.MustFromBig(tipCap),
		Gas:       swapTx.Gas(),
		To:        swapTx.To(),
		Value:     uint256.MustFromBig(swapTx.Value()),
		Data:      swapTx.Data(),
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

func (s *Scenario) MintErc20AndWethForChildWallets() (map[common.Address][]error, error) {
	if s.options.MaxWallets == 0 {
		return nil, fmt.Errorf("max wallets not set")
	}

	client := s.tester.GetClient(tester.SelectByIndex, 0)

	rootWallet := s.tester.GetRootWallet()
	rootWalletTransactor, err := rootWallet.GetTransactor(true, big.NewInt(0))
	if err != nil {
		s.logger.Errorf("could not get transactor for root wallet: %v", err)
		return nil, err
	}

	tokenMintAmount := s.tokenMintAmount
	batchSize := uint64(100)
	batchIndex := uint64(0)

	errorMapLock := sync.Mutex{}
	errorMap := make(map[common.Address][]error)

	wg := sync.WaitGroup{}
	// batch up the mints and deposits in order to not overwhelm the rpc
	for {
		wg.Add(1)
		go func(batchIndex uint64, batchSize uint64, errorMap *map[common.Address][]error, errorMapLock *sync.Mutex) {
			defer wg.Done()
			finalBatchIndex := batchIndex + batchSize

			s.logger.Infof("funding child wallets: %v/%v", batchIndex, s.tester.GetTotalChildWallets())

			wg1 := sync.WaitGroup{}

			for {
				if batchIndex > uint64(s.tester.GetTotalChildWallets()) || batchIndex >= finalBatchIndex {
					break
				}

				childWallet := s.tester.GetWallet(tester.SelectByIndex, int(batchIndex))

				wg1.Add(1)
				go func(errorMap *map[common.Address][]error, errorMapLock *sync.Mutex) {
					defer wg1.Done()
					// get child wallet transactor
					childWalletTransactor, err := childWallet.GetTransactor(true, big.NewInt(0))
					if err != nil {
						s.logger.Errorf("could not get transactor for child wallet: %v", err)
						errorMapLock.Lock()
						(*errorMap)[childWallet.GetAddress()] = append((*errorMap)[childWallet.GetAddress()], err)
						errorMapLock.Unlock()
						return
					}

					customizableErc20Contract, err := s.GetCustomizableErc20Contract()
					if err != nil {
						s.logger.Errorf("could not create Dai contract: %v", err)
						errorMapLock.Lock()
						(*errorMap)[childWallet.GetAddress()] = append((*errorMap)[childWallet.GetAddress()], err)
						errorMapLock.Unlock()
						return
					}

					wethContract, err := s.GetWethContract()
					if err != nil {
						s.logger.Errorf("could not create WETH contract: %v", err)
						errorMapLock.Lock()
						(*errorMap)[childWallet.GetAddress()] = append((*errorMap)[childWallet.GetAddress()], err)
						errorMapLock.Unlock()
						return
					}

					// transfer erc20 for child wallet
					transferTx, err := customizableErc20Contract.Transfer(rootWalletTransactor, childWallet.GetAddress(), tokenMintAmount)
					if err != nil {
						s.logger.Errorf("could not transfer Erc20 for child wallet: %v", err)
						errorMapLock.Lock()
						(*errorMap)[childWallet.GetAddress()] = append((*errorMap)[childWallet.GetAddress()], err)
						errorMapLock.Unlock()
						return
					}

					wethTransferTx, err := wethContract.Transfer(rootWalletTransactor, childWallet.GetAddress(), tokenMintAmount)
					if err != nil {
						s.logger.Errorf("could not transfer WETH for child wallet: %v", err)
						errorMapLock.Lock()
						(*errorMap)[childWallet.GetAddress()] = append((*errorMap)[childWallet.GetAddress()], err)
						errorMapLock.Unlock()
						return
					}

					childWalletTransactor.Value = big.NewInt(0)
					// dai approval
					approveTx, err := customizableErc20Contract.Approve(childWalletTransactor, s.swapRouterContract, tokenMintAmount)
					if err != nil {
						s.logger.Errorf("could not approve DAI for child wallet: %v", err)
						errorMapLock.Lock()
						(*errorMap)[childWallet.GetAddress()] = append((*errorMap)[childWallet.GetAddress()], err)
						errorMapLock.Unlock()
						return
					}

					wethApproveTx, err := wethContract.Approve(childWalletTransactor, s.swapRouterContract, tokenMintAmount)
					if err != nil {
						s.logger.Errorf("could not approve WETH for child wallet: %v", err)
						errorMapLock.Lock()
						(*errorMap)[childWallet.GetAddress()] = append((*errorMap)[childWallet.GetAddress()], err)
						errorMapLock.Unlock()
						return
					}

					internalWg := sync.WaitGroup{}

					internalWg.Add(1)
					// send and await txs
					go func(errorMap *map[common.Address][]error, errorMapLock *sync.Mutex) {
						defer internalWg.Done()
						_, _, err = txbuilder.SendAndAwaitTx(txbuilder.SendTxOpts{
							Gas:     0,
							Wallet:  rootWallet,
							Tx:      transferTx,
							Client:  client,
							BaseFee: int64(s.options.BaseFee),
							TipFee:  int64(s.options.TipFee),
						})
						if err != nil {
							s.logger.Errorf("could not mint DAI for child wallet: %v", err)
							errorMapLock.Lock()
							(*errorMap)[childWallet.GetAddress()] = append((*errorMap)[childWallet.GetAddress()], err)
							errorMapLock.Unlock()
							return
						}
						_, _, err = txbuilder.SendAndAwaitTx(txbuilder.SendTxOpts{
							Gas:     0,
							Wallet:  rootWallet,
							Tx:      wethTransferTx,
							Client:  client,
							BaseFee: int64(s.options.BaseFee),
							TipFee:  int64(s.options.TipFee),
						})
						if err != nil {
							s.logger.Errorf("could not transfer WETH for child wallet: %v", err)
							errorMapLock.Lock()
							(*errorMap)[childWallet.GetAddress()] = append((*errorMap)[childWallet.GetAddress()], err)
							errorMapLock.Unlock()
							return
						}

						_, _, err = txbuilder.SendAndAwaitTx(txbuilder.SendTxOpts{
							Gas:     0,
							Wallet:  rootWallet,
							Tx:      wethApproveTx,
							Client:  client,
							BaseFee: int64(s.options.BaseFee),
							TipFee:  int64(s.options.TipFee),
						})
						if err != nil {
							s.logger.Errorf("could not approve WETH for child wallet: %v", err)
							errorMapLock.Lock()
							(*errorMap)[childWallet.GetAddress()] = append((*errorMap)[childWallet.GetAddress()], err)
							errorMapLock.Unlock()
							return
						}

						_, _, err = txbuilder.SendAndAwaitTx(txbuilder.SendTxOpts{
							Gas:     0,
							Wallet:  rootWallet,
							Tx:      approveTx,
							Client:  client,
							BaseFee: int64(s.options.BaseFee),
							TipFee:  int64(s.options.TipFee),
						})
						if err != nil {
							s.logger.Errorf("could not approve ERC20 for child wallet: %v", err)
							errorMapLock.Lock()
							(*errorMap)[childWallet.GetAddress()] = append((*errorMap)[childWallet.GetAddress()], err)
							errorMapLock.Unlock()
							return
						}
					}(errorMap, errorMapLock)

					//internalWg.Add(1)
					//go func(errorMap *map[common.Address][]error, errorMapLock *sync.Mutex) {
					//	defer internalWg.Done()
					//	_, _, err = txbuilder.SendAndAwaitTx(txbuilder.SendTxOpts{
					//		Gas:     0,
					//		Wallet:  childWallet,
					//		Tx:      approveTx,
					//		Client:  client,
					//		BaseFee: int64(s.options.BaseFee),
					//		TipFee:  int64(s.options.TipFee),
					//	})
					//	if err != nil {
					//		s.logger.Errorf("could not approve DAI for child wallet: %v", err)
					//		errorMapLock.Lock()
					//		(*errorMap)[childWallet.GetAddress()] = append((*errorMap)[childWallet.GetAddress()], err)
					//		errorMapLock.Unlock()
					//		return
					//	}
					//}(errorMap, errorMapLock)

					internalWg.Wait()
				}(errorMap, errorMapLock)

				wg1.Wait()

				batchIndex += 1
			}

		}(batchIndex, batchSize, &errorMap, &errorMapLock)

		batchIndex += batchSize

		// we are done if this is true
		if batchIndex >= uint64(s.tester.GetTotalChildWallets()) {
			break
		}
	}

	wg.Wait()

	s.logger.Infof("minted DAI for child wallets")

	return errorMap, nil
}

func (s *Scenario) awaitTx(txIdx uint64, tx *types.Transaction, client *txbuilder.Client, wallet *txbuilder.Wallet) {
	var awaitConfirmation bool = true
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

	s.logger.WithField("client", client.GetName()).Infof(" transaction %d confirmed in block #%v with %s. total fee: %v gwei (base: %v, blob: %v)", txIdx+1, blockNum, txStatus, gweiTotalFee, gweiBaseFee, gweiBlobFee)
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

func (s *Scenario) GetWethBalance(wallet *txbuilder.Wallet) (*big.Int, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	wethContract, err := univ3swaptx.NewWeth(s.wethContract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return wethContract.BalanceOf(nil, wallet.GetAddress())
}

func (s *Scenario) GetCustomizableErc20Balance(wallet *txbuilder.Wallet) (*big.Int, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	customizableErc20, err := univ3swaptx.NewCustomizableErc20(s.customizableErc20Contract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return customizableErc20.BalanceOf(nil, wallet.GetAddress())
}

func (s *Scenario) GetWethAllowance(wallet *txbuilder.Wallet, to common.Address) (*big.Int, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	wethContract, err := univ2tx.NewWeth(s.wethContract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return wethContract.Allowance(nil, wallet.GetAddress(), to)
}

func (s *Scenario) GetCustomizableErc20Allowance(wallet *txbuilder.Wallet, to common.Address) (*big.Int, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	customizableErc20, err := univ3swaptx.NewCustomizableErc20(s.customizableErc20Contract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return customizableErc20.Allowance(nil, wallet.GetAddress(), to)
}

func (s *Scenario) GetCustomizableErc20Contract() (*univ3swaptx.CustomizableErc20, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	customizableErc20Contract, err := univ3swaptx.NewCustomizableErc20(s.customizableErc20Contract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return customizableErc20Contract, nil
}

func (s *Scenario) GetWethContract() (*univ3swaptx.Weth, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	wethContract, err := univ3swaptx.NewWeth(s.wethContract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return wethContract, nil
}

func (s *Scenario) GetSwapRouterContract() (*univ3swaptx.SwapRouter, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	swapRouterContract, err := univ3swaptx.NewSwapRouter(s.swapRouterContract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return swapRouterContract, nil
}
