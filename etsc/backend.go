// Copyright 2014 The go-etsc Authors
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

// Package etsc implements the etsc protocol.
package etsc

import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/ETSC3259/etsc/accounts"
	"github.com/ETSC3259/etsc/common"
	"github.com/ETSC3259/etsc/common/hexutil"
	"github.com/ETSC3259/etsc/consensus"
	"github.com/ETSC3259/etsc/consensus/clique"
	"github.com/ETSC3259/etsc/consensus/etschash"
	"github.com/ETSC3259/etsc/core"
	"github.com/ETSC3259/etsc/core/bloombits"
	"github.com/ETSC3259/etsc/core/rawdb"
	"github.com/ETSC3259/etsc/core/types"
	"github.com/ETSC3259/etsc/core/vm"
	"github.com/ETSC3259/etsc/etsc/downloader"
	"github.com/ETSC3259/etsc/etsc/filters"
	"github.com/ETSC3259/etsc/etsc/gasprice"
	"github.com/ETSC3259/etsc/etscdb"
	"github.com/ETSC3259/etsc/event"
	"github.com/ETSC3259/etsc/internal/etscapi"
	"github.com/ETSC3259/etsc/log"
	"github.com/ETSC3259/etsc/miner"
	"github.com/ETSC3259/etsc/node"
	"github.com/ETSC3259/etsc/p2p"
	"github.com/ETSC3259/etsc/params"
	"github.com/ETSC3259/etsc/rlp"
	"github.com/ETSC3259/etsc/rpc"
)

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
	SetBloomBitsIndexer(bbIndexer *core.ChainIndexer)
}

// etsc implements the etsc full node service.
type etsc struct {
	config      *Config
	chainConfig *params.ChainConfig

	// Channel for shutting down the service
	shutdownChan chan bool // Channel for shutting down the etsc

	// Handlers
	txPool          *core.TxPool
	blockchain      *core.BlockChain
	protocolManager *ProtocolManager
	lesServer       LesServer

	// DB interfaces
	chainDb etscdb.Database // Block chain database

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	APIBackend *EtscAPIBackend

	miner     *miner.Miner
	gasPrice  *big.Int
	etscbase common.Address

	networkID     uint64
	netRPCService *etscapi.PublicNetAPI

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and etscbase)
}

func (s *etsc) AddLesServer(ls LesServer) {
	s.lesServer = ls
	ls.SetBloomBitsIndexer(s.bloomIndexer)
}

// New creates a new etsc object (including the
// initialisation of the common etsc object)
func New(ctx *node.ServiceContext, config *Config) (*etsc, error) {
	// Ensure configuration values are compatible and sane
	if config.SyncMode == downloader.LightSync {
		return nil, errors.New("can't run etsc.etsc in light sync mode, use les.Lightetsc")
	}
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}
	if config.MinerGasPrice == nil || config.MinerGasPrice.Cmp(common.Big0) <= 0 {
		log.Warn("Sanitizing invalid miner gas price", "provided", config.MinerGasPrice, "updated", DefaultConfig.MinerGasPrice)
		config.MinerGasPrice = new(big.Int).Set(DefaultConfig.MinerGasPrice)
	}
	// Assemble the etsc object
	chainDb, err := CreateDB(ctx, config, "chaindata")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, config.Genesis)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	etsc := &etsc{
		config:         config,
		chainDb:        chainDb,
		chainConfig:    chainConfig,
		eventMux:       ctx.EventMux,
		accountManager: ctx.AccountManager,
		engine:         CreateConsensusEngine(ctx, chainConfig, &config.Etschash, config.MinerNotify, config.MinerNoverify, chainDb),
		shutdownChan:   make(chan bool),
		networkID:      config.NetworkId,
		gasPrice:       config.MinerGasPrice,
		etscbase:      config.Etscbase,
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   NewBloomIndexer(chainDb, params.BloomBitsBlocks, params.BloomConfirms),
	}

	log.Info("Initialising etsc protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	if !config.SkipBcVersionCheck {
		bcVersion := rawdb.ReadDatabaseVersion(chainDb)
		if bcVersion != core.BlockChainVersion && bcVersion != 0 {
			return nil, fmt.Errorf("Blockchain DB version mismatch (%d / %d).\n", bcVersion, core.BlockChainVersion)
		}
		rawdb.WriteDatabaseVersion(chainDb, core.BlockChainVersion)
	}
	var (
		vmConfig = vm.Config{
			EnablePreimageRecording: config.EnablePreimageRecording,
			EWASMInterpreter:        config.EWASMInterpreter,
			EVMInterpreter:          config.EVMInterpreter,
		}
		cacheConfig = &core.CacheConfig{Disabled: config.NoPruning, TrieNodeLimit: config.TrieCache, TrieTimeLimit: config.TrieTimeout}
	)
	etsc.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, etsc.chainConfig, etsc.engine, vmConfig, etsc.shouldPreserve)
	if err != nil {
		return nil, err
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		etsc.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}
	etsc.bloomIndexer.Start(etsc.blockchain)

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = ctx.ResolvePath(config.TxPool.Journal)
	}
	etsc.txPool = core.NewTxPool(config.TxPool, etsc.chainConfig, etsc.blockchain)

	if etsc.protocolManager, err = NewProtocolManager(etsc.chainConfig, config.SyncMode, config.NetworkId, etsc.eventMux, etsc.txPool, etsc.engine, etsc.blockchain, chainDb); err != nil {
		return nil, err
	}

	etsc.miner = miner.New(etsc, etsc.chainConfig, etsc.EventMux(), etsc.engine, config.MinerRecommit, config.MinerGasFloor, config.MinerGasCeil, etsc.isLocalBlock)
	etsc.miner.SetExtra(makeExtraData(config.MinerExtraData))

	etsc.APIBackend = &EtscAPIBackend{etsc, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.MinerGasPrice
	}
	etsc.APIBackend.gpo = gasprice.NewOracle(etsc.APIBackend, gpoParams)

	return etsc, nil
}

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]interface{}{
			uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
			"getsc",
			runtime.Version(),
			runtime.GOOS,
		})
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.MaximumExtraDataSize)
		extra = nil
	}
	return extra
}

// CreateDB creates the chain database.
func CreateDB(ctx *node.ServiceContext, config *Config, name string) (etscdb.Database, error) {
	db, err := ctx.OpenDatabase(name, config.DatabaseCache, config.DatabaseHandles)
	if err != nil {
		return nil, err
	}
	if db, ok := db.(*etscdb.LDBDatabase); ok {
		db.Meter("etsc/db/chaindata/")
	}
	return db, nil
}

// CreateConsensusEngine creates the required type of consensus engine instance for an etsc service
func CreateConsensusEngine(ctx *node.ServiceContext, chainConfig *params.ChainConfig, config *etschash.Config, notify []string, noverify bool, db etscdb.Database) consensus.Engine {
	// If proof-of-authority is requested, set it up
	if chainConfig.Clique != nil {
		return clique.New(chainConfig.Clique, db)
	}
	// Otherwise assume proof-of-work
	switch config.PowMode {
	case etschash.ModeFake:
		log.Warn("Etschash used in fake mode")
		return etschash.NewFaker()
	case etschash.ModeTest:
		log.Warn("Etschash used in test mode")
		return etschash.NewTester(nil, noverify)
	case etschash.ModeShared:
		log.Warn("Etschash used in shared mode")
		return etschash.NewShared()
	default:
		engine := etschash.New(etschash.Config{
			CacheDir:       ctx.ResolvePath(config.CacheDir),
			CachesInMem:    config.CachesInMem,
			CachesOnDisk:   config.CachesOnDisk,
			DatasetDir:     config.DatasetDir,
			DatasetsInMem:  config.DatasetsInMem,
			DatasetsOnDisk: config.DatasetsOnDisk,
		}, notify, noverify)
		engine.SetThreads(-1) // Disable CPU mining
		return engine
	}
}

// APIs return the collection of RPC services the etsc package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *etsc) APIs() []rpc.API {
	apis := etscapi.GetAPIs(s.APIBackend)

	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, s.engine.APIs(s.BlockChain())...)

	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "etsc",
			Version:   "1.0",
			Service:   NewPublicetscAPI(s),
			Public:    true,
		}, {
			Namespace: "etsc",
			Version:   "1.0",
			Service:   NewPublicMinerAPI(s),
			Public:    true,
		}, {
			Namespace: "etsc",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "miner",
			Version:   "1.0",
			Service:   NewPrivateMinerAPI(s),
			Public:    false,
		}, {
			Namespace: "etsc",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.APIBackend, false),
			Public:    true,
		}, {
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(s),
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(s),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPrivateDebugAPI(s.chainConfig, s),
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)
}

func (s *etsc) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *etsc) Etscbase() (eb common.Address, err error) {
	s.lock.RLock()
	etscbase := s.etscbase
	s.lock.RUnlock()

	if etscbase != (common.Address{}) {
		return etscbase, nil
	}
	if wallets := s.AccountManager().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			etscbase := accounts[0].Address

			s.lock.Lock()
			s.etscbase = etscbase
			s.lock.Unlock()

			log.Info("Etscbase automatically configured", "address", etscbase)
			return etscbase, nil
		}
	}
	return common.Address{}, fmt.Errorf("etscbase must be explicitly specified")
}

// isLocalBlock checks whether the specified block is mined
// by local miner accounts.
//
// We regard two types of accounts as local miner account: etscbase
// and accounts specified via `txpool.locals` flag.
func (s *etsc) isLocalBlock(block *types.Block) bool {
	author, err := s.engine.Author(block.Header())
	if err != nil {
		log.Warn("Failed to retrieve block author", "number", block.NumberU64(), "hash", block.Hash(), "err", err)
		return false
	}
	// Check whether the given address is etscbase.
	s.lock.RLock()
	etscbase := s.etscbase
	s.lock.RUnlock()
	if author == etscbase {
		return true
	}
	// Check whether the given address is specified by `txpool.local`
	// CLI flag.
	for _, account := range s.config.TxPool.Locals {
		if account == author {
			return true
		}
	}
	return false
}

// shouldPreserve checks whether we should preserve the given block
// during the chain reorg depending on whether the author of block
// is a local account.
func (s *etsc) shouldPreserve(block *types.Block) bool {
	// The reason we need to disable the self-reorg preserving for clique
	// is it can be probable to introduce a deadlock.
	//
	// e.g. If there are 7 available signers
	//
	// r1   A
	// r2     B
	// r3       C
	// r4         D
	// r5   A      [X] F G
	// r6    [X]
	//
	// In the round5, the inturn signer E is offline, so the worst case
	// is A, F and G sign the block of round5 and reject the block of opponents
	// and in the round6, the last available signer B is offline, the whole
	// network is stuck.
	if _, ok := s.engine.(*clique.Clique); ok {
		return false
	}
	return s.isLocalBlock(block)
}

// SetEtscbase sets the mining reward address.
func (s *etsc) SetEtscbase(etscbase common.Address) {
	s.lock.Lock()
	s.etscbase = etscbase
	s.lock.Unlock()

	s.miner.SetEtscbase(etscbase)
}

// StartMining starts the miner with the given number of CPU threads. If mining
// is already running, this method adjust the number of threads allowed to use
// and updates the minimum price required by the transaction pool.
func (s *etsc) StartMining(threads int) error {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}
	if th, ok := s.engine.(threaded); ok {
		log.Info("Updated mining threads", "threads", threads)
		if threads == 0 {
			threads = -1 // Disable the miner from within
		}
		th.SetThreads(threads)
	}
	// If the miner was not running, initialize it
	if !s.IsMining() {
		// Propagate the initial price point to the transaction pool
		s.lock.RLock()
		price := s.gasPrice
		s.lock.RUnlock()
		s.txPool.SetGasPrice(price)

		// Configure the local mining address
		eb, err := s.Etscbase()
		if err != nil {
			log.Error("Cannot start mining without etscbase", "err", err)
			return fmt.Errorf("etscbase missing: %v", err)
		}
		if clique, ok := s.engine.(*clique.Clique); ok {
			wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
			if wallet == nil || err != nil {
				log.Error("Etscbase account unavailable locally", "err", err)
				return fmt.Errorf("signer missing: %v", err)
			}
			clique.Authorize(eb, wallet.SignHash)
		}
		// If mining is started, we can disable the transaction rejection mechanism
		// introduced to speed sync times.
		atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)

		go s.miner.Start(eb)
	}
	return nil
}

// StopMining terminates the miner, both at the consensus engine level as well as
// at the block creation level.
func (s *etsc) StopMining() {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}
	if th, ok := s.engine.(threaded); ok {
		th.SetThreads(-1)
	}
	// Stop the block creating itself
	s.miner.Stop()
}

func (s *etsc) IsMining() bool      { return s.miner.Mining() }
func (s *etsc) Miner() *miner.Miner { return s.miner }

func (s *etsc) AccountManager() *accounts.Manager  { return s.accountManager }
func (s *etsc) BlockChain() *core.BlockChain       { return s.blockchain }
func (s *etsc) TxPool() *core.TxPool               { return s.txPool }
func (s *etsc) EventMux() *event.TypeMux           { return s.eventMux }
func (s *etsc) Engine() consensus.Engine           { return s.engine }
func (s *etsc) ChainDb() etscdb.Database            { return s.chainDb }
func (s *etsc) IsListening() bool                  { return true } // Always listening
func (s *etsc) EtscVersion() int                    { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *etsc) NetVersion() uint64                 { return s.networkID }
func (s *etsc) Downloader() *downloader.Downloader { return s.protocolManager.downloader }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *etsc) Protocols() []p2p.Protocol {
	if s.lesServer == nil {
		return s.protocolManager.SubProtocols
	}
	return append(s.protocolManager.SubProtocols, s.lesServer.Protocols()...)
}

// Start implements node.Service, starting all internal goroutines needed by the
// etsc protocol implementation.
func (s *etsc) Start(srvr *p2p.Server) error {
	// Start the bloom bits servicing goroutines
	s.startBloomHandlers(params.BloomBitsBlocks)

	// Start the RPC service
	s.netRPCService = etscapi.NewPublicNetAPI(srvr, s.NetVersion())

	// Figure out a max peers count based on the server limits
	maxPeers := srvr.MaxPeers
	if s.config.LightServ > 0 {
		if s.config.LightPeers >= srvr.MaxPeers {
			return fmt.Errorf("invalid peer config: light peer count (%d) >= total peer count (%d)", s.config.LightPeers, srvr.MaxPeers)
		}
		maxPeers -= s.config.LightPeers
	}
	// Start the networking layer and the light server if requested
	s.protocolManager.Start(maxPeers)
	if s.lesServer != nil {
		s.lesServer.Start(srvr)
	}
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// etsc protocol.
func (s *etsc) Stop() error {
	s.bloomIndexer.Close()
	s.blockchain.Stop()
	s.engine.Close()
	s.protocolManager.Stop()
	if s.lesServer != nil {
		s.lesServer.Stop()
	}
	s.txPool.Stop()
	s.miner.Stop()
	s.eventMux.Stop()

	s.chainDb.Close()
	close(s.shutdownChan)
	return nil
}
