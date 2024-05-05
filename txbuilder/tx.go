package txbuilder

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"math/big"
)

type SendTxOpts struct {
	Gas     uint64
	Wallet  *Wallet
	Tx      *types.Transaction
	Client  *Client
	BaseFee int64
	TipFee  int64
}

func SendAndAwaitTx(opts SendTxOpts) (*types.Receipt, *Client, error) {

	var feeCap *big.Int
	var tipCap *big.Int

	if opts.BaseFee > 0 {
		feeCap = new(big.Int).Mul(big.NewInt(opts.BaseFee), big.NewInt(1000000000))
	}
	if opts.TipFee > 0 {
		tipCap = new(big.Int).Mul(big.NewInt(opts.TipFee), big.NewInt(1000000000))
	}

	if feeCap == nil || tipCap == nil {
		var err error
		feeCap, tipCap, err = opts.Client.GetSuggestedFee()
		if err != nil {
			return nil, opts.Client, err
		}
	}

	if feeCap.Cmp(big.NewInt(1000000000)) < 0 {
		feeCap = big.NewInt(1000000000)
	}
	if tipCap.Cmp(big.NewInt(1000000000)) < 0 {
		tipCap = big.NewInt(1000000000)
	}

	gas := opts.Tx.Gas()
	if opts.Gas != 0 {
		gas = opts.Gas
	}

	dynamicTxData, err := DynFeeTx(&TxMetadata{
		GasFeeCap: uint256.MustFromBig(feeCap),
		GasTipCap: uint256.MustFromBig(tipCap),
		Gas:       gas,
		To:        opts.Tx.To(),
		Value:     uint256.MustFromBig(opts.Tx.Value()),
		Data:      opts.Tx.Data(),
	})
	if err != nil {
		return nil, nil, err
	}
	finalTx, err := opts.Wallet.BuildDynamicFeeTx(dynamicTxData)
	if err != nil {
		return nil, nil, err
	}

	err = opts.Client.SendTransaction(finalTx)
	if err != nil {
		return nil, opts.Client, err
	}

	receipt, _, err := opts.Client.AwaitTransaction(finalTx)
	if err != nil {
		return nil, opts.Client, err
	}

	return receipt, opts.Client, nil
}
