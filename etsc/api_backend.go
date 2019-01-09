// Copyright 2015 The go-etsc Authors
// This file is part of the go-etsc library.
//
// The go-etsc library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-etsc library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-etsc library. If not, see <http://www.gnu.org/licenses/>.

package etsc

import (
	"context"
	"math/big"

	"github.com/ETSC3259/etsc/accounts"
	"github.com/ETSC3259/etsc/common"
	"github.com/ETSC3259/etsc/common/math"
	"github.com/ETSC3259/etsc/core"
	"github.com/ETSC3259/etsc/core/bloombits"
	"github.com/ETSC3259/etsc/core/state"
	"github.com/ETSC3259/etsc/core/types"
	"github.com/ETSC3259/etsc/core/vm"
	"github.com/ETSC3259/etsc/etsc/downloader"
	"github.com/ETSC3259/etsc/etsc/gasprice"
	"github.com/ETSC3259/etsc/etscdb"
	"github.com/ETSC3259/etsc/event"
	"github.com/ETSC3259/etsc/params"
	"github.com/ETSC3259/etsc/rpc"
)

// EtscAPIBackend implements etscapi.Backend for full nodes
type EtscAPIBackend struct {
	etsc *etsc
	gpo *gasprice.Oracle
}

// ChainConfig returns the active chain configuration.
func (b *EtscAPIBackend) ChainConfig() *params.ChainConfig {
	return b.etsc.chainConfig
}

func (b *EtscAPIBackend) CurrentBlock() *types.Block {
	return b.etsc.blockchain.CurrentBlock()
}

func (b *EtscAPIBackend) SetHead(number uint64) {
	b.etsc.protocolManager.downloader.Cancel()
	b.etsc.blockchain.SetHead(number)
}

func (b *EtscAPIBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.etsc.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.etsc.blockchain.CurrentBlock().Header(), nil
	}
	return b.etsc.blockchain.getsceaderByNumber(uint64(blockNr)), nil
}

func (b *EtscAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.etsc.blockchain.getsceaderByHash(hash), nil
}

func (b *EtscAPIBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.etsc.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.etsc.blockchain.CurrentBlock(), nil
	}
	return b.etsc.blockchain.GetBlockByNumber(uint64(blockNr)), nil
}

func (b *EtscAPIBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block, state := b.etsc.miner.Pending()
		return state, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := b.etsc.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *EtscAPIBackend) GetBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.etsc.blockchain.GetBlockByHash(hash), nil
}

func (b *EtscAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.etsc.blockchain.GetReceiptsByHash(hash), nil
}

func (b *EtscAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	receipts := b.etsc.blockchain.GetReceiptsByHash(hash)
	if receipts == nil {
		return nil, nil
	}
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (b *EtscAPIBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.etsc.blockchain.GetTdByHash(blockHash)
}

func (b *EtscAPIBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header, vmCfg vm.Config) (*vm.EVM, func() error, error) {
	state.SetBalance(msg.From(), math.MaxBig256)
	vmError := func() error { return nil }

	context := core.NewEVMContext(msg, header, b.etsc.BlockChain(), nil)
	return vm.NewEVM(context, state, b.etsc.chainConfig, vmCfg), vmError, nil
}

func (b *EtscAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.etsc.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *EtscAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.etsc.BlockChain().SubscribeChainEvent(ch)
}

func (b *EtscAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.etsc.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *EtscAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.etsc.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *EtscAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.etsc.BlockChain().SubscribeLogsEvent(ch)
}

func (b *EtscAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.etsc.txPool.AddLocal(signedTx)
}

func (b *EtscAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.etsc.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *EtscAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.etsc.txPool.Get(hash)
}

func (b *EtscAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.etsc.txPool.State().GetNonce(addr), nil
}

func (b *EtscAPIBackend) Stats() (pending int, queued int) {
	return b.etsc.txPool.Stats()
}

func (b *EtscAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.etsc.TxPool().Content()
}

func (b *EtscAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.etsc.TxPool().SubscribeNewTxsEvent(ch)
}

func (b *EtscAPIBackend) Downloader() *downloader.Downloader {
	return b.etsc.Downloader()
}

func (b *EtscAPIBackend) ProtocolVersion() int {
	return b.etsc.EtscVersion()
}

func (b *EtscAPIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}

func (b *EtscAPIBackend) ChainDb() etscdb.Database {
	return b.etsc.ChainDb()
}

func (b *EtscAPIBackend) EventMux() *event.TypeMux {
	return b.etsc.EventMux()
}

func (b *EtscAPIBackend) AccountManager() *accounts.Manager {
	return b.etsc.AccountManager()
}

func (b *EtscAPIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.etsc.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

func (b *EtscAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.etsc.bloomRequests)
	}
}
