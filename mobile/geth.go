// Copyright 2016 The go-etsc Authors
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

// Contains all the wrappers from the node package to support client side node
// management on mobile platforms.

package getsc

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ETSC3259/etsc/core"
	"github.com/ETSC3259/etsc/etsc"
	"github.com/ETSC3259/etsc/etsc/downloader"
	"github.com/ETSC3259/etsc/etscclient"
	"github.com/ETSC3259/etsc/etscstats"
	"github.com/ETSC3259/etsc/internal/debug"
	"github.com/ETSC3259/etsc/les"
	"github.com/ETSC3259/etsc/node"
	"github.com/ETSC3259/etsc/p2p"
	"github.com/ETSC3259/etsc/p2p/nat"
	"github.com/ETSC3259/etsc/params"
	whisper "github.com/ETSC3259/etsc/whisper/whisperv6"
)

// NodeConfig represents the collection of configuration values to fine tune the getsc
// node embedded into a mobile process. The available values are a subset of the
// entire API provided by go-etsc to reduce the maintenance surface and dev
// complexity.
type NodeConfig struct {
	// Bootstrap nodes used to establish connectivity with the rest of the network.
	BootstrapNodes *Enodes

	// MaxPeers is the maximum number of peers that can be connected. If this is
	// set to zero, then only the configured static and trusted peers can connect.
	MaxPeers int

	// etscEnabled specifies whether the node should run the etsc protocol.
	etscEnabled bool

	// etscNetworkID is the network identifier used by the etsc protocol to
	// decide if remote peers should be accepted or not.
	etscNetworkID int64 // uint64 in truth, but Java can't handle that...

	// etscGenesis is the genesis JSON to use to seed the blockchain with. An
	// empty genesis state is equivalent to using the mainnet's state.
	etscGenesis string

	// etscDatabaseCache is the system memory in MB to allocate for database caching.
	// A minimum of 16MB is always reserved.
	etscDatabaseCache int

	// etscNetStats is a netstats connection string to use to report various
	// chain, transaction and node stats to a monitoring server.
	//
	// It has the form "nodename:secret@host:port"
	etscNetStats string

	// WhisperEnabled specifies whether the node should run the Whisper protocol.
	WhisperEnabled bool

	// Listening address of pprof server.
	PprofAddress string
}

// defaultNodeConfig contains the default node configuration values to use if all
// or some fields are missing from the user's specified list.
var defaultNodeConfig = &NodeConfig{
	BootstrapNodes:        FoundationBootnodes(),
	MaxPeers:              25,
	etscEnabled:       true,
	etscNetworkID:     1,
	etscDatabaseCache: 16,
}

// NewNodeConfig creates a new node option set, initialized to the default values.
func NewNodeConfig() *NodeConfig {
	config := *defaultNodeConfig
	return &config
}

// Node represents a getsc etsc node instance.
type Node struct {
	node *node.Node
}

// NewNode creates and configures a new getsc node.
func NewNode(datadir string, config *NodeConfig) (stack *Node, _ error) {
	// If no or partial configurations were specified, use defaults
	if config == nil {
		config = NewNodeConfig()
	}
	if config.MaxPeers == 0 {
		config.MaxPeers = defaultNodeConfig.MaxPeers
	}
	if config.BootstrapNodes == nil || config.BootstrapNodes.Size() == 0 {
		config.BootstrapNodes = defaultNodeConfig.BootstrapNodes
	}

	if config.PprofAddress != "" {
		debug.StartPProf(config.PprofAddress)
	}

	// Create the empty networking stack
	nodeConf := &node.Config{
		Name:        clientIdentifier,
		Version:     params.VersionWithMeta,
		DataDir:     datadir,
		KeyStoreDir: filepath.Join(datadir, "keystore"), // Mobile should never use internal keystores!
		P2P: p2p.Config{
			NoDiscovery:      true,
			DiscoveryV5:      true,
			BootstrapNodesV5: config.BootstrapNodes.nodes,
			ListenAddr:       ":0",
			NAT:              nat.Any(),
			MaxPeers:         config.MaxPeers,
		},
	}
	rawStack, err := node.New(nodeConf)
	if err != nil {
		return nil, err
	}

	debug.Memsize.Add("node", rawStack)

	var genesis *core.Genesis
	if config.etscGenesis != "" {
		// Parse the user supplied genesis spec if not mainnet
		genesis = new(core.Genesis)
		if err := json.Unmarshal([]byte(config.etscGenesis), genesis); err != nil {
			return nil, fmt.Errorf("invalid genesis spec: %v", err)
		}
		// If we have the testnet, hard code the chain configs too
		if config.etscGenesis == TestnetGenesis() {
			genesis.Config = params.TestnetChainConfig
			if config.etscNetworkID == 1 {
				config.etscNetworkID = 3
			}
		}
	}
	// Register the etsc protocol if requested
	if config.etscEnabled {
		etscConf := etsc.DefaultConfig
		etscConf.Genesis = genesis
		etscConf.SyncMode = downloader.LightSync
		etscConf.NetworkId = uint64(config.etscNetworkID)
		etscConf.DatabaseCache = config.etscDatabaseCache
		if err := rawStack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
			return les.New(ctx, &etscConf)
		}); err != nil {
			return nil, fmt.Errorf("etsc init: %v", err)
		}
		// If netstats reporting is requested, do it
		if config.etscNetStats != "" {
			if err := rawStack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
				var lesServ *les.Lightetsc
				ctx.Service(&lesServ)

				return etscstats.New(config.etscNetStats, nil, lesServ)
			}); err != nil {
				return nil, fmt.Errorf("netstats init: %v", err)
			}
		}
	}
	// Register the Whisper protocol if requested
	if config.WhisperEnabled {
		if err := rawStack.Register(func(*node.ServiceContext) (node.Service, error) {
			return whisper.New(&whisper.DefaultConfig), nil
		}); err != nil {
			return nil, fmt.Errorf("whisper init: %v", err)
		}
	}
	return &Node{rawStack}, nil
}

// Start creates a live P2P node and starts running it.
func (n *Node) Start() error {
	return n.node.Start()
}

// Stop terminates a running node along with all it's services. If the node was
// not started, an error is returned.
func (n *Node) Stop() error {
	return n.node.Stop()
}

// GetetscClient retrieves a client to access the etsc subsystem.
func (n *Node) GetetscClient() (client *etscClient, _ error) {
	rpc, err := n.node.Attach()
	if err != nil {
		return nil, err
	}
	return &etscClient{etscclient.NewClient(rpc)}, nil
}

// GetNodeInfo gathers and returns a collection of metadata known about the host.
func (n *Node) GetNodeInfo() *NodeInfo {
	return &NodeInfo{n.node.Server().NodeInfo()}
}

// GetPeersInfo returns an array of metadata objects describing connected peers.
func (n *Node) GetPeersInfo() *PeerInfos {
	return &PeerInfos{n.node.Server().PeersInfo()}
}
