package univ2tx

import (
	"context"
	rand2 "crypto/rand"
	"flag"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	univ2tx "github.com/ethpandaops/goomy-blob/scenarios/univ2tx/contracts"
	"github.com/ethpandaops/goomy-blob/scenariotypes"
	"github.com/ethpandaops/goomy-blob/tester"
	"github.com/ethpandaops/goomy-blob/txbuilder"
	"github.com/ethpandaops/goomy-blob/utils"
	"github.com/holiman/uint256"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"math/big"
	"os"
	"sync"
	"time"
)

type ScenarioOptions struct {
	TotalCount         uint64
	Throughput         uint64
	MaxPending         uint64
	MaxWallets         uint64
	Timeout            uint64
	BaseFee            uint64
	TipFee             uint64
	TokenMintAmount    uint64 // this is in eth values
	AmountToSwap       uint64
	RandomAmountToSwap bool
}

type Scenario struct {
	options ScenarioOptions
	logger  *logrus.Entry
	tester  *tester.Tester

	uniswapFactoryContract common.Address
	wethContract           common.Address
	daiContract            common.Address
	uniswapRouterContract  common.Address
	pairContract           common.Address

	pendingCount  uint64
	pendingChan   chan bool
	pendingWGroup sync.WaitGroup

	tokenMintAmount uint64
}

func NewScenario() scenariotypes.Scenario {
	return &Scenario{
		logger: logrus.WithField("scenario", "univ2tx"),
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
	flag.Uint64Var(&s.options.TokenMintAmount, "token-mint-amount", 1000000000000000000, "Amount of tokens to mint for each child wallet")
	flag.Uint64Var(&s.options.AmountToSwap, "amount-to-swap", 1, "Amount of tokens to swap in each transaction(in gwei)")
	flag.BoolVar(&s.options.RandomAmountToSwap, "random-amount-to-swap", false, "Randomize the amount of tokens to swap in each transaction(in gwei)")

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

	if s.options.TokenMintAmount > 0 {
		s.tokenMintAmount = s.options.TokenMintAmount
	}

	return nil
}

func (s *Scenario) Setup(testerCfg *tester.Tester) error {
	s.tester = testerCfg
	rootWallet := s.tester.GetRootWallet()

	s.logger.Infof("starting scenario: univ2tx")
	s.logger.Infof("setting up uniswap v2 contracts...")

	s.logger.Infof("deploying uniswap V2 Factory...")
	receipt, _, err := s.DeployUniswapV2Factory(rootWallet)
	if err != nil {
		s.logger.Errorf("could not deploy univ2 factory contract: %v", err)
		return err
	}

	s.uniswapFactoryContract = receipt.ContractAddress
	s.logger.Infof("deployed uniswap V2 Factory at %v", receipt.ContractAddress.String())

	s.logger.Infof("deploying weth contract...")
	receipt, _, err = s.DeployWeth(rootWallet)
	if err != nil {
		s.logger.Errorf("could not deploy weth contract: %v", err)
		return err
	}

	s.wethContract = receipt.ContractAddress
	s.logger.Infof("deployed weth contract at %v", receipt.ContractAddress.String())

	s.logger.Infof("deploying dai contract...")
	receipt, _, err = s.DeployDai(rootWallet)
	if err != nil {
		s.logger.Errorf("could not deploy dai contract: %v", err)
		return err
	}

	s.daiContract = receipt.ContractAddress
	s.logger.Infof("deployed dai contract at %v", receipt.ContractAddress.String())

	s.logger.Infof("deploying uniswap V2 Router...")

	receipt, _, err = s.DeployUniswapV2Router(rootWallet, s.uniswapFactoryContract, s.wethContract)
	if err != nil {
		s.logger.Errorf("could not deploy univ2 router contract: %v", err)
		return err
	}

	s.uniswapRouterContract = receipt.ContractAddress
	s.logger.Infof("deployed uniswap V2 Router at %v", receipt.ContractAddress.String())

	s.logger.Infof("deploying uniswap V2 Pair...")
	receipt, _, pairAddr, err := s.CreateUniswapV2Pair(rootWallet, s.daiContract, s.wethContract, s.uniswapFactoryContract)
	if err != nil {
		s.logger.Errorf("could not create univ2 pair: %v", err)
		return err
	}

	s.pairContract = pairAddr
	s.logger.Infof("created uniswap V2 Pair: %v", pairAddr.String())

	// mint dai and weth for root wallet
	s.logger.Infof("minting DAI and WETH for root wallet...")
	err = s.MintDaiAndWethForRootWallet()
	if err != nil {
		s.logger.Errorf("could not mint DAI and WETH for root wallet: %v", err)
		return err
	}

	// get dai and weth balances of root wallet
	daiBalance, err := s.GetDaiBalance(rootWallet)
	if err != nil {
		s.logger.Errorf("could not get DAI balance for root wallet: %v", err)
		return err
	}
	wethBalance, err := s.GetWethBalance(rootWallet)
	if err != nil {
		s.logger.Errorf("could not get WETH balance for root wallet: %v", err)
		return err
	}
	s.logger.Infof("root wallet has DAI balance of %v and WETH balance of %v", utils.WeiToEther(uint256.MustFromBig(daiBalance)), utils.WeiToEther(uint256.MustFromBig(wethBalance)))

	// add the entire dai and weth balance of root wallet as liquidity to the pool
	s.logger.Infof("adding liquidity to the pool...")
	receipt, _, err = s.AddLiquidity(rootWallet, daiBalance, wethBalance)
	if err != nil {
		s.logger.Errorf("could not add liquidity to the pool: %v", err)
		return err
	}

	// get pool reserves
	daiReserve, wethReserve, err := s.GetPairReserves()
	if err != nil {
		s.logger.Errorf("could not get pair reserves: %v", err)
		return err
	}
	s.logger.Infof("pair reserves: DAI: %v, WETH: %v", utils.WeiToEther(uint256.MustFromBig(daiReserve)), utils.WeiToEther(uint256.MustFromBig(wethReserve)))

	// now we need to mint DAI and WETH for all child wallets
	s.logger.Infof("minting DAI and WETH for child wallets...")
	errorMap, err := s.MintDaiAndWethForChildWallets()
	if err != nil {
		s.logger.Errorf("could not mint DAI and WETH for child wallets: %v", err)
		return err
	}
	if len(errorMap) > 0 {
		for addr, err := range errorMap {
			s.logger.Errorf("could not mint DAI and WETH for child wallet %v: %v", addr.String(), err)
		}
		return fmt.Errorf("could not mint DAI and WETH for all child wallets")
	}

	// get dai and weth balances of each child wallet
	for i := 0; i < s.tester.GetTotalChildWallets(); i++ {
		childWallet := s.tester.GetWallet(tester.SelectByIndex, i)
		daiBalance, err := s.GetDaiBalance(childWallet)
		if err != nil {
			s.logger.Errorf("could not get DAI balance for child wallet %v: %v", childWallet.GetAddress().String(), err)
			return err
		}
		wethBalance, err := s.GetWethBalance(childWallet)
		if err != nil {
			s.logger.Errorf("could not get WETH balance for child wallet %v: %v", childWallet.GetAddress().String(), err)
			return err
		}
		s.logger.Infof("child wallet %v has DAI balance of %v and WETH balance of %v", childWallet.GetAddress().String(), utils.WeiToEther(uint256.MustFromBig(daiBalance)), utils.WeiToEther(uint256.MustFromBig(wethBalance)))
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

	// get pair reserves
	daiReserve, wethReserve, err := s.GetPairReserves()
	if err != nil {
		s.logger.Errorf("could not get pair reserves: %v", err)
		return err
	}

	s.logger.Infof("pair reserves after swap load: DAI: %v, WETH: %v", utils.WeiToEther(uint256.MustFromBig(daiReserve)), utils.WeiToEther(uint256.MustFromBig(wethReserve)))

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

	routerContract, err := s.GetRouterContract()
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
		swapDirection = []common.Address{s.daiContract, s.wethContract}
	} else {
		swapDirection = []common.Address{s.wethContract, s.daiContract}
	}

	// get amount to swap
	amount := uint256.NewInt(s.options.AmountToSwap)
	amount = amount.Mul(amount, uint256.NewInt(1000000000))
	if s.options.RandomAmountToSwap {
		n, err := rand2.Int(rand2.Reader, amount.ToBig())
		if err == nil {
			amount = uint256.MustFromBig(n)
		}
	}

	walletTransactor, err := s.GetTransactor(wallet, true, big.NewInt(0))
	if err != nil {
		return nil, nil, err
	}

	// get time 10mins from now
	deadline := time.Now().Add(10 * time.Minute).Unix()
	swapTx, err := routerContract.SwapExactTokensForTokens(walletTransactor, amount.ToBig(), big.NewInt(0), swapDirection, wallet.GetAddress(), big.NewInt(deadline))
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

	err = client.SendTransaction(tx)
	if err != nil {
		return nil, client, err
	}

	s.pendingWGroup.Add(1)
	go s.awaitTx(txIdx, tx, client, wallet)

	return tx, client, nil
}

func (s *Scenario) MintDaiAndWethForRootWallet() error {
	wallet := s.tester.GetRootWallet()

	daiAmountToMint := big.NewInt(0).Mul(big.NewInt(int64(s.tester.GetTotalChildWallets())), big.NewInt(int64(s.tokenMintAmount)))
	wethAmountToMint := big.NewInt(0).Mul(big.NewInt(int64(s.tester.GetTotalChildWallets())), big.NewInt(int64(s.tokenMintAmount)))

	s.logger.Infof("Minting DAI and WETH for root wallet: %v", wallet.GetAddress())

	rootWalletTransactor, err := s.GetTransactor(wallet, true, big.NewInt(0))
	if err != nil {
		s.logger.Errorf("could not get transactor for root wallet: %v", err)
		return err
	}

	daiContract, err := s.GetDaiContract()
	if err != nil {
		s.logger.Errorf("could not create Dai contract: %v", err)
		return err
	}
	wethContract, err := s.GetWethContract()
	if err != nil {
		s.logger.Errorf("could not create Weth contract: %v", err)
		return err
	}

	// mint DAI for child wallet
	daiMintTx, err := daiContract.Mint(rootWalletTransactor, wallet.GetAddress(), daiAmountToMint)
	if err != nil {
		s.logger.Errorf("could not mint DAI for root wallet: %v", err)
		return err
	}

	// Deposit Weth for child wallet
	rootWalletTransactor.Value = wethAmountToMint
	wethDepositTx, err := wethContract.Deposit(rootWalletTransactor)
	if err != nil {
		s.logger.Errorf("could not deposit WETH for root wallet: %v", err)
		return err
	}

	rootWalletTransactor.Value = big.NewInt(0)
	// dai approval
	daiApproveTx, err := daiContract.Approve(rootWalletTransactor, s.uniswapRouterContract, daiAmountToMint)
	if err != nil {
		s.logger.Errorf("could not approve DAI for root wallet: %v", err)
		return err
	}
	// weth approval
	wethApproveTx, err := wethContract.Approve(rootWalletTransactor, s.uniswapRouterContract, wethAmountToMint)
	if err != nil {
		s.logger.Errorf("could not approve WETH for root wallet: %v", err)
		return err
	}

	// send and await txs
	receipt, _, err := s.SendAndAwaitTx(wallet, daiMintTx, SendTxOpts{Gas: 0})
	if err != nil {
		s.logger.Errorf("could not mint DAI for root wallet: %v", err)
		return err
	}
	s.logger.Infof("Minted DAI for root wallet: %v at block: %d", wallet.GetAddress(), receipt.BlockNumber)

	receipt, _, err = s.SendAndAwaitTx(wallet, wethDepositTx, SendTxOpts{Gas: 0})
	if err != nil {
		s.logger.Errorf("could not deposit WETH for root wallet: %v", err)
		return err
	}
	s.logger.Infof("Minted Weth for root wallet: %v at block: %d", wallet.GetAddress(), receipt.BlockNumber)

	receipt, _, err = s.SendAndAwaitTx(wallet, daiApproveTx, SendTxOpts{Gas: 0})
	if err != nil {
		s.logger.Errorf("could not approve DAI for root wallet: %v", err)
		return err
	}
	s.logger.Infof("Approved Dai to router contract for root wallet: %v at block: %d", wallet.GetAddress(), receipt.BlockNumber)

	receipt, _, err = s.SendAndAwaitTx(wallet, wethApproveTx, SendTxOpts{Gas: 0})
	if err != nil {
		s.logger.Errorf("could not approve WETH for root wallet: %v", err)
		return err
	}
	s.logger.Infof("Approved Weth to router contract for root wallet: %v at block: %d", wallet.GetAddress(), receipt.BlockNumber)

	return nil
}

func (s *Scenario) MintDaiAndWethForChildWallets() (map[common.Address]error, error) {
	if s.options.MaxWallets == 0 {
		return nil, fmt.Errorf("max wallets not set")
	}

	wg := sync.WaitGroup{}
	walletIdx := 0
	rootWallet := s.tester.GetRootWallet()

	tokenMintAmount := s.tokenMintAmount
	errorMap := map[common.Address]error{}

	for {
		childWallet := s.tester.GetWallet(tester.SelectByIndex, walletIdx)
		walletIdx += 1

		wg.Add(1)

		go func(childWallet *txbuilder.Wallet) {
			defer wg.Done()

			s.logger.Infof("Minting DAI and WETH for child wallet: %v", childWallet.GetAddress())

			// get child wallet transactor
			childWalletTransactor, err := s.GetTransactor(childWallet, true, big.NewInt(0))
			if err != nil {
				s.logger.Errorf("could not get transactor for child wallet: %v", err)
				errorMap[childWallet.GetAddress()] = err
				return
			}
			rootWalletTransactor, err := s.GetTransactor(rootWallet, true, big.NewInt(0))
			if err != nil {
				s.logger.Errorf("could not get transactor for root wallet: %v", err)
				errorMap[childWallet.GetAddress()] = err
				return
			}

			daiContract, err := s.GetDaiContract()
			if err != nil {
				s.logger.Errorf("could not create Dai contract: %v", err)
				errorMap[childWallet.GetAddress()] = err
				return
			}
			wethContract, err := s.GetWethContract()
			if err != nil {
				s.logger.Errorf("could not create Weth contract: %v", err)
				errorMap[childWallet.GetAddress()] = err
				return
			}

			// mint DAI for child wallet
			daiMintTx, err := daiContract.Mint(rootWalletTransactor, childWallet.GetAddress(), big.NewInt(int64(tokenMintAmount)))
			if err != nil {
				s.logger.Errorf("could not mint DAI for child wallet: %v", err)
				errorMap[childWallet.GetAddress()] = err
				return
			}
			// Deposit Weth for child wallet
			childWalletTransactor.Value = big.NewInt(int64(tokenMintAmount))
			wethDepositTx, err := wethContract.Deposit(childWalletTransactor)
			if err != nil {
				s.logger.Errorf("could not deposit WETH for child wallet: %v", err)
				errorMap[childWallet.GetAddress()] = err
				return
			}

			childWalletTransactor.Value = big.NewInt(0)
			// dai approval
			daiApproveTx, err := daiContract.Approve(childWalletTransactor, s.uniswapRouterContract, big.NewInt(int64(tokenMintAmount)))
			if err != nil {
				s.logger.Errorf("could not approve DAI for child wallet: %v", err)
				errorMap[childWallet.GetAddress()] = err
				return
			}
			// weth approval
			wethApproveTx, err := wethContract.Approve(childWalletTransactor, s.uniswapRouterContract, big.NewInt(int64(tokenMintAmount)))
			if err != nil {
				s.logger.Errorf("could not approve WETH for child wallet: %v", err)
				errorMap[childWallet.GetAddress()] = err
				return
			}

			// send and await txs
			receipt, _, err := s.SendAndAwaitTx(rootWallet, daiMintTx, SendTxOpts{Gas: 0})
			if err != nil {
				s.logger.Errorf("could not mint DAI for child wallet: %v", err)
				errorMap[childWallet.GetAddress()] = err
				return
			}
			s.logger.Infof("Minted DAI for child wallet: %v at block: %d", childWallet.GetAddress(), receipt.BlockNumber)

			receipt, _, err = s.SendAndAwaitTx(childWallet, wethDepositTx, SendTxOpts{Gas: 0})
			if err != nil {
				s.logger.Errorf("could not deposit WETH for child wallet: %v", err)
				errorMap[childWallet.GetAddress()] = err
				return
			}
			s.logger.Infof("Minted Weth for child wallet: %v at block: %d", childWallet.GetAddress(), receipt.BlockNumber)

			receipt, _, err = s.SendAndAwaitTx(childWallet, daiApproveTx, SendTxOpts{Gas: 0})
			if err != nil {
				s.logger.Errorf("could not approve DAI for child wallet: %v", err)
				errorMap[childWallet.GetAddress()] = err
				return
			}
			s.logger.Infof("Approved Dai to router contract for child wallet: %v at block: %d", childWallet.GetAddress(), receipt.BlockNumber)

			receipt, _, err = s.SendAndAwaitTx(childWallet, wethApproveTx, SendTxOpts{Gas: 0})
			if err != nil {
				s.logger.Errorf("could not approve WETH for child wallet: %v", err)
				errorMap[childWallet.GetAddress()] = err
				return
			}
			s.logger.Infof("Approved Weth to router contract for child wallet: %v at block: %d", childWallet.GetAddress(), receipt.BlockNumber)
		}(childWallet)

		if walletIdx >= s.tester.GetTotalChildWallets() {
			break
		}
	}
	wg.Wait()

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

func (s *Scenario) DeployUniswapV2Factory(wallet *txbuilder.Wallet) (*types.Receipt, *txbuilder.Client, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	transactor, err := s.GetTransactor(wallet, true, big.NewInt(0))
	if err != nil {
		return nil, nil, err
	}

	_, deployTx, _, err := univ2tx.DeployUniswapV2Factory(transactor, client.GetEthClient(), wallet.GetAddress())
	if err != nil {
		return nil, nil, err
	}

	return s.SendAndAwaitTx(wallet, deployTx, SendTxOpts{Gas: 6000000})
}

func (s *Scenario) CreateUniswapV2Pair(wallet *txbuilder.Wallet, tokenA common.Address, tokenB common.Address, factory common.Address) (*types.Receipt, *txbuilder.Client, common.Address, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	transactor, err := s.GetTransactor(wallet, true, nil)
	if err != nil {
		return nil, nil, common.Address{}, err
	}

	factoryContract, err := univ2tx.NewUniswapV2Factory(factory, client.GetEthClient())
	if err != nil {
		return nil, nil, common.Address{}, err
	}

	s.logger.Infof("Creating pair for %v and %v", tokenA.String(), tokenB.String())
	tx, err := factoryContract.CreatePair(transactor, tokenA, tokenB)
	if err != nil {
		return nil, nil, common.Address{}, err
	}

	receipt, _, err := s.SendAndAwaitTx(wallet, tx, SendTxOpts{Gas: 0})
	if err != nil {
		return nil, client, common.Address{}, err
	}

	pairAddr, err := factoryContract.GetPair(nil, tokenA, tokenB)
	if err != nil {
		return nil, client, common.Address{}, err
	}

	return receipt, client, pairAddr, nil
}

func (s *Scenario) DeployWeth(wallet *txbuilder.Wallet) (*types.Receipt, *txbuilder.Client, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	transactor, err := s.GetTransactor(wallet, true, big.NewInt(0))
	if err != nil {
		return nil, nil, err
	}

	_, deployTx, _, err := univ2tx.DeployWeth(transactor, client.GetEthClient())
	if err != nil {
		return nil, nil, err
	}

	return s.SendAndAwaitTx(wallet, deployTx, SendTxOpts{Gas: 6000000})
}

func (s *Scenario) DeployDai(wallet *txbuilder.Wallet) (*types.Receipt, *txbuilder.Client, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	transactor, err := s.GetTransactor(wallet, true, big.NewInt(0))
	if err != nil {
		return nil, nil, err
	}

	chainId, err := client.GetChainId()
	if err != nil {
		return nil, nil, err
	}

	_, deployTx, _, err := univ2tx.DeployDai(transactor, client.GetEthClient(), chainId)
	if err != nil {
		return nil, nil, err
	}

	return s.SendAndAwaitTx(wallet, deployTx, SendTxOpts{Gas: 6000000})
}

func (s *Scenario) AddLiquidity(wallet *txbuilder.Wallet, daiAmount *big.Int, wethAmount *big.Int) (*types.Receipt, *txbuilder.Client, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	transactor, err := s.GetTransactor(wallet, true, big.NewInt(0))
	if err != nil {
		return nil, nil, err
	}

	router, err := univ2tx.NewUniswapV2Router(s.uniswapRouterContract, client.GetEthClient())
	if err != nil {
		return nil, client, err
	}

	timestamp := time.Now().Add(10 * time.Minute).Unix()

	transactor.NoSend = true
	tx, err := router.AddLiquidity(transactor, s.daiContract, s.wethContract, daiAmount, wethAmount, big.NewInt(0), big.NewInt(0), wallet.GetAddress(), big.NewInt(timestamp))
	if err != nil {
		return nil, client, err
	}

	return s.SendAndAwaitTx(wallet, tx, SendTxOpts{Gas: 500000})
}

func (s *Scenario) Swap(tokenA common.Address, tokenB common.Address, amountIn *big.Int) (*types.Receipt, *txbuilder.Client, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)
	wallet := s.tester.GetWallet(tester.SelectByIndex, 0)

	transactor, err := s.GetTransactor(wallet, true, big.NewInt(0))
	if err != nil {
		return nil, nil, err
	}

	router, err := univ2tx.NewUniswapV2Router(s.uniswapRouterContract, client.GetEthClient())
	if err != nil {
		return nil, client, err
	}

	// timestamp 60s from now
	timestamp := time.Now().Add(10 * time.Minute).Unix()
	tx, err := router.SwapExactTokensForTokens(transactor, amountIn, big.NewInt(0), []common.Address{tokenA, tokenB}, wallet.GetAddress(), big.NewInt(timestamp))
	if err != nil {
		return nil, client, err
	}

	receipt, _, err := s.SendAndAwaitTx(wallet, tx, SendTxOpts{Gas: 1000000})
	if err != nil {
		s.logger.Infof("Erroring out here!")
		return nil, client, err
	}

	s.logger.Infof("tx hash is %v", receipt.TxHash.String())

	return receipt, client, nil
}

func (s *Scenario) DeployUniswapV2Router(wallet *txbuilder.Wallet, factoryAddress common.Address, wethAddress common.Address) (*types.Receipt, *txbuilder.Client, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	transactor, err := s.GetTransactor(wallet, true, big.NewInt(0))
	if err != nil {
		return nil, nil, err
	}

	_, deployTx, _, err := univ2tx.DeployUniswapV2Router(transactor, client.GetEthClient(), factoryAddress, wethAddress)
	if err != nil {
		return nil, nil, err
	}

	return s.SendAndAwaitTx(wallet, deployTx, SendTxOpts{Gas: 6000000})
}

func (s *Scenario) GetWethBalance(wallet *txbuilder.Wallet) (*big.Int, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	wethContract, err := univ2tx.NewWeth(s.wethContract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return wethContract.BalanceOf(nil, wallet.GetAddress())
}

func (s *Scenario) GetDaiBalance(wallet *txbuilder.Wallet) (*big.Int, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	daiContract, err := univ2tx.NewDai(s.daiContract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return daiContract.BalanceOf(nil, wallet.GetAddress())
}

func (s *Scenario) GetWethAllowance(wallet *txbuilder.Wallet, to common.Address) (*big.Int, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	wethContract, err := univ2tx.NewWeth(s.wethContract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return wethContract.Allowance(nil, wallet.GetAddress(), to)
}

func (s *Scenario) GetDaiAllowance(wallet *txbuilder.Wallet, to common.Address) (*big.Int, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	daiContract, err := univ2tx.NewDai(s.daiContract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return daiContract.Allowance(nil, wallet.GetAddress(), to)
}

func (s *Scenario) GetPairReserves() (*big.Int, *big.Int, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	pairContract, err := univ2tx.NewUniswapV2Pair(s.pairContract, client.GetEthClient())
	if err != nil {
		return nil, nil, err
	}

	reserves, err := pairContract.GetReserves(nil)
	if err != nil {
		return nil, nil, err
	}

	return reserves.Reserve0, reserves.Reserve1, nil
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

type SendTxOpts struct {
	Gas uint64
}

func (s *Scenario) SendAndAwaitTx(wallet *txbuilder.Wallet, tx *types.Transaction, opts SendTxOpts) (*types.Receipt, *txbuilder.Client, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

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

	gas := tx.Gas()
	if opts.Gas != 0 {
		gas = opts.Gas
	}

	dynamicTxData, err := txbuilder.DynFeeTx(&txbuilder.TxMetadata{
		GasFeeCap: uint256.MustFromBig(feeCap),
		GasTipCap: uint256.MustFromBig(tipCap),
		Gas:       gas,
		To:        tx.To(),
		Value:     uint256.NewInt(tx.Value().Uint64()),
		Data:      tx.Data(),
	})
	if err != nil {
		return nil, nil, err
	}
	finalTx, err := wallet.BuildDynamicFeeTx(dynamicTxData)
	if err != nil {
		return nil, nil, err
	}

	err = client.SendTransaction(finalTx)
	if err != nil {
		return nil, client, err
	}

	receipt, _, err := client.AwaitTransaction(finalTx)
	if err != nil {
		return nil, client, err
	}

	return receipt, client, nil
}

func (s *Scenario) GetDaiContract() (*univ2tx.Dai, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	daiContract, err := univ2tx.NewDai(s.daiContract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return daiContract, nil
}

func (s *Scenario) GetWethContract() (*univ2tx.Weth, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	wethContract, err := univ2tx.NewWeth(s.wethContract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return wethContract, nil
}

func (s *Scenario) GetRouterContract() (*univ2tx.UniswapV2Router, error) {
	client := s.tester.GetClient(tester.SelectByIndex, 0)

	routerContract, err := univ2tx.NewUniswapV2Router(s.uniswapRouterContract, client.GetEthClient())
	if err != nil {
		return nil, err
	}

	return routerContract, nil
}
