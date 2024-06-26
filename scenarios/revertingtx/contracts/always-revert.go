// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// RevertingtxMetaData contains all meta data concerning the Revertingtx contract.
var RevertingtxMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"name\":\"alwaysRevert\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x6080604052348015600f57600080fd5b5060b780601d6000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c80639fb3785314602d575b600080fd5b60336035565b005b60405162461bcd60e51b815260206004820152601c60248201527f546869732066756e6374696f6e20616c77617973207265766572747300000000604482015260640160405180910390fdfea26469706673582212206749d3ec93dd02d035ed0ef96b2fb34f9f3e0ab7973ff58f09801e86c2acc8b464736f6c63430008190033",
}

// RevertingtxABI is the input ABI used to generate the binding from.
// Deprecated: Use RevertingtxMetaData.ABI instead.
var RevertingtxABI = RevertingtxMetaData.ABI

// RevertingtxBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use RevertingtxMetaData.Bin instead.
var RevertingtxBin = RevertingtxMetaData.Bin

// DeployRevertingtx deploys a new Ethereum contract, binding an instance of Revertingtx to it.
func DeployRevertingtx(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Revertingtx, error) {
	parsed, err := RevertingtxMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(RevertingtxBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Revertingtx{RevertingtxCaller: RevertingtxCaller{contract: contract}, RevertingtxTransactor: RevertingtxTransactor{contract: contract}, RevertingtxFilterer: RevertingtxFilterer{contract: contract}}, nil
}

// Revertingtx is an auto generated Go binding around an Ethereum contract.
type Revertingtx struct {
	RevertingtxCaller     // Read-only binding to the contract
	RevertingtxTransactor // Write-only binding to the contract
	RevertingtxFilterer   // Log filterer for contract events
}

// RevertingtxCaller is an auto generated read-only Go binding around an Ethereum contract.
type RevertingtxCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RevertingtxTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RevertingtxTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RevertingtxFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type RevertingtxFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RevertingtxSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RevertingtxSession struct {
	Contract     *Revertingtx      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RevertingtxCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RevertingtxCallerSession struct {
	Contract *RevertingtxCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// RevertingtxTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RevertingtxTransactorSession struct {
	Contract     *RevertingtxTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// RevertingtxRaw is an auto generated low-level Go binding around an Ethereum contract.
type RevertingtxRaw struct {
	Contract *Revertingtx // Generic contract binding to access the raw methods on
}

// RevertingtxCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RevertingtxCallerRaw struct {
	Contract *RevertingtxCaller // Generic read-only contract binding to access the raw methods on
}

// RevertingtxTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RevertingtxTransactorRaw struct {
	Contract *RevertingtxTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRevertingtx creates a new instance of Revertingtx, bound to a specific deployed contract.
func NewRevertingtx(address common.Address, backend bind.ContractBackend) (*Revertingtx, error) {
	contract, err := bindRevertingtx(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Revertingtx{RevertingtxCaller: RevertingtxCaller{contract: contract}, RevertingtxTransactor: RevertingtxTransactor{contract: contract}, RevertingtxFilterer: RevertingtxFilterer{contract: contract}}, nil
}

// NewRevertingtxCaller creates a new read-only instance of Revertingtx, bound to a specific deployed contract.
func NewRevertingtxCaller(address common.Address, caller bind.ContractCaller) (*RevertingtxCaller, error) {
	contract, err := bindRevertingtx(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &RevertingtxCaller{contract: contract}, nil
}

// NewRevertingtxTransactor creates a new write-only instance of Revertingtx, bound to a specific deployed contract.
func NewRevertingtxTransactor(address common.Address, transactor bind.ContractTransactor) (*RevertingtxTransactor, error) {
	contract, err := bindRevertingtx(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &RevertingtxTransactor{contract: contract}, nil
}

// NewRevertingtxFilterer creates a new log filterer instance of Revertingtx, bound to a specific deployed contract.
func NewRevertingtxFilterer(address common.Address, filterer bind.ContractFilterer) (*RevertingtxFilterer, error) {
	contract, err := bindRevertingtx(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &RevertingtxFilterer{contract: contract}, nil
}

// bindRevertingtx binds a generic wrapper to an already deployed contract.
func bindRevertingtx(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := RevertingtxMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Revertingtx *RevertingtxRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Revertingtx.Contract.RevertingtxCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Revertingtx *RevertingtxRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Revertingtx.Contract.RevertingtxTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Revertingtx *RevertingtxRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Revertingtx.Contract.RevertingtxTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Revertingtx *RevertingtxCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Revertingtx.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Revertingtx *RevertingtxTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Revertingtx.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Revertingtx *RevertingtxTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Revertingtx.Contract.contract.Transact(opts, method, params...)
}

// AlwaysRevert is a paid mutator transaction binding the contract method 0x9fb37853.
//
// Solidity: function alwaysRevert() returns()
func (_Revertingtx *RevertingtxTransactor) AlwaysRevert(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Revertingtx.contract.Transact(opts, "alwaysRevert")
}

// AlwaysRevert is a paid mutator transaction binding the contract method 0x9fb37853.
//
// Solidity: function alwaysRevert() returns()
func (_Revertingtx *RevertingtxSession) AlwaysRevert() (*types.Transaction, error) {
	return _Revertingtx.Contract.AlwaysRevert(&_Revertingtx.TransactOpts)
}

// AlwaysRevert is a paid mutator transaction binding the contract method 0x9fb37853.
//
// Solidity: function alwaysRevert() returns()
func (_Revertingtx *RevertingtxTransactorSession) AlwaysRevert() (*types.Transaction, error) {
	return _Revertingtx.Contract.AlwaysRevert(&_Revertingtx.TransactOpts)
}
