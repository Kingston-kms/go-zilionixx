package gossip

import (
	"fmt"
	"math/big"
	"math/rand"
	"sync"

	"github.com/Fantom-foundation/lachesis-base/hash"
	"github.com/Fantom-foundation/lachesis-base/inter/dag"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/Fantom-foundation/lachesis-base/lachesis"
	"github.com/Fantom-foundation/lachesis-base/utils/workers"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	notify "github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discv5"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/Fantom-foundation/go-zilionixx/ethapi"
	"github.com/Fantom-foundation/go-zilionixx/eventcheck"
	"github.com/Fantom-foundation/go-zilionixx/eventcheck/basiccheck"
	"github.com/Fantom-foundation/go-zilionixx/eventcheck/epochcheck"
	"github.com/Fantom-foundation/go-zilionixx/eventcheck/gaspowercheck"
	"github.com/Fantom-foundation/go-zilionixx/eventcheck/heavycheck"
	"github.com/Fantom-foundation/go-zilionixx/eventcheck/parentscheck"
	"github.com/Fantom-foundation/go-zilionixx/evmcore"
	"github.com/Fantom-foundation/go-zilionixx/gossip/blockproc"
	"github.com/Fantom-foundation/go-zilionixx/gossip/blockproc/drivermodule"
	"github.com/Fantom-foundation/go-zilionixx/gossip/blockproc/eventmodule"
	"github.com/Fantom-foundation/go-zilionixx/gossip/blockproc/evmmodule"
	"github.com/Fantom-foundation/go-zilionixx/gossip/blockproc/sealmodule"
	"github.com/Fantom-foundation/go-zilionixx/gossip/blockproc/verwatcher"
	"github.com/Fantom-foundation/go-zilionixx/gossip/emitter"
	"github.com/Fantom-foundation/go-zilionixx/gossip/filters"
	"github.com/Fantom-foundation/go-zilionixx/gossip/gasprice"
	"github.com/Fantom-foundation/go-zilionixx/inter"
	"github.com/Fantom-foundation/go-zilionixx/logger"
	"github.com/Fantom-foundation/go-zilionixx/opera"
	"github.com/Fantom-foundation/go-zilionixx/utils/wgmutex"
	"github.com/Fantom-foundation/go-zilionixx/valkeystore"
	"github.com/Fantom-foundation/go-zilionixx/vecmt"
)

type ServiceFeed struct {
	scope notify.SubscriptionScope

	newEpoch        notify.Feed
	newPack         notify.Feed
	newEmittedEvent notify.Feed
	newBlock        notify.Feed
	newTxs          notify.Feed
	newLogs         notify.Feed
}

func (f *ServiceFeed) SubscribeNewEpoch(ch chan<- idx.Epoch) notify.Subscription {
	return f.scope.Track(f.newEpoch.Subscribe(ch))
}

func (f *ServiceFeed) SubscribeNewEmitted(ch chan<- *inter.EventPayload) notify.Subscription {
	return f.scope.Track(f.newEmittedEvent.Subscribe(ch))
}

func (f *ServiceFeed) SubscribeNewBlock(ch chan<- evmcore.ChainHeadNotify) notify.Subscription {
	return f.scope.Track(f.newBlock.Subscribe(ch))
}

func (f *ServiceFeed) SubscribeNewTxs(ch chan<- core.NewTxsEvent) notify.Subscription {
	return f.scope.Track(f.newTxs.Subscribe(ch))
}

func (f *ServiceFeed) SubscribeNewLogs(ch chan<- []*types.Log) notify.Subscription {
	return f.scope.Track(f.newLogs.Subscribe(ch))
}

type BlockProc struct {
	SealerModule        blockproc.SealerModule
	TxListenerModule    blockproc.TxListenerModule
	GenesisTxTransactor blockproc.TxTransactor
	PreTxTransactor     blockproc.TxTransactor
	PostTxTransactor    blockproc.TxTransactor
	EventsModule        blockproc.ConfirmedEventsModule
	EVMModule           blockproc.EVM
}

func DefaultBlockProc(g opera.Genesis) BlockProc {
	return BlockProc{
		SealerModule:        sealmodule.New(),
		TxListenerModule:    drivermodule.NewDriverTxListenerModule(),
		GenesisTxTransactor: drivermodule.NewDriverTxGenesisTransactor(g),
		PreTxTransactor:     drivermodule.NewDriverTxPreTransactor(),
		PostTxTransactor:    drivermodule.NewDriverTxTransactor(),
		EventsModule:        eventmodule.New(),
		EVMModule:           evmmodule.New(),
	}
}

// Service implements go-ethereum/node.Service interface.
type Service struct {
	config Config

	wg   sync.WaitGroup
	done chan struct{}

	// server
	p2pServer *p2p.Server
	Name      string
	Topic     discv5.Topic

	serverPool *serverPool

	accountManager *accounts.Manager

	// application
	store               *Store
	engine              lachesis.Consensus
	dagIndexer          *vecmt.Index
	engineMu            *sync.RWMutex
	emitter             *emitter.Emitter
	txpool              *evmcore.TxPool
	heavyCheckReader    HeavyCheckReader
	gasPowerCheckReader GasPowerCheckReader
	checkers            *eventcheck.Checkers
	uniqueEventIDs      uniqueID

	// version watcher
	verWatcher *verwatcher.VerWarcher

	blockProcWg        sync.WaitGroup
	blockProcTasks     *workers.Workers
	blockProcTasksDone chan struct{}
	blockProcModules   BlockProc

	blockBusyFlag uint32
	eventBusyFlag uint32

	feed ServiceFeed

	// application protocol
	pm *ProtocolManager

	EthAPI        *EthAPIBackend
	netRPCService *ethapi.PublicNetAPI

	stopped bool

	logger.Instance
}

func NewService(stack *node.Node, config Config, store *Store, signer valkeystore.SignerI, blockProc BlockProc, engine lachesis.Consensus, dagIndexer *vecmt.Index) (*Service, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = stack.ResolvePath(config.TxPool.Journal)
	}

	svc, err := newService(config, store, signer, blockProc, engine, dagIndexer)
	if err != nil {
		return nil, err
	}

	svc.p2pServer = stack.Server()
	svc.accountManager = stack.AccountManager()
	// Create the net API service
	svc.netRPCService = ethapi.NewPublicNetAPI(svc.p2pServer, store.GetRules().NetworkID)

	return svc, nil
}

func newService(config Config, store *Store, signer valkeystore.SignerI, blockProc BlockProc, engine lachesis.Consensus, dagIndexer *vecmt.Index) (*Service, error) {
	svc := &Service{
		config:             config,
		done:               make(chan struct{}),
		blockProcTasksDone: make(chan struct{}),
		Name:               fmt.Sprintf("Node-%d", rand.Int()),
		store:              store,
		engine:             engine,
		blockProcModules:   blockProc,
		dagIndexer:         dagIndexer,
		engineMu:           new(sync.RWMutex),
		uniqueEventIDs:     uniqueID{new(big.Int)},
		Instance:           logger.MakeInstance(),
	}

	svc.blockProcTasks = workers.New(new(sync.WaitGroup), svc.blockProcTasksDone, 1)

	// create server pool
	trustedNodes := []string{}
	svc.serverPool = newServerPool(store.async.table.Peers, svc.done, &svc.wg, trustedNodes)

	// create tx pool
	net := store.GetRules()
	stateReader := svc.GetEvmStateReader()
	svc.txpool = evmcore.NewTxPool(config.TxPool, net.EvmChainConfig(), stateReader)

	// create checkers
	svc.heavyCheckReader.Addrs.Store(NewEpochPubKeys(svc.store, svc.store.GetEpoch()))                                             // read pub keys of current epoch from disk
	svc.gasPowerCheckReader.Ctx.Store(NewGasPowerContext(svc.store, svc.store.GetValidators(), svc.store.GetEpoch(), net.Economy)) // read gaspower check data from disk
	svc.checkers = makeCheckers(config.HeavyCheck, net.EvmChainConfig().ChainID, &svc.heavyCheckReader, &svc.gasPowerCheckReader, svc.store)

	// create protocol manager
	var err error
	svc.pm, err = NewProtocolManager(config, &svc.feed, svc.txpool, svc.engineMu, svc.checkers, store, svc.processEvent, svc.serverPool)
	if err != nil {
		return nil, err
	}

	// create API backend
	svc.EthAPI = &EthAPIBackend{config.ExtRPCEnabled, svc, stateReader, nil}
	svc.EthAPI.gpo = gasprice.NewOracle(svc.EthAPI, svc.config.GPO)

	// load epoch DB
	svc.store.loadEpochStore(svc.store.GetEpoch())
	es := svc.store.getEpochStore(svc.store.GetEpoch())
	svc.dagIndexer.Reset(svc.store.GetValidators(), es.table.DagIndex, func(id hash.Event) dag.Event {
		return svc.store.GetEvent(id)
	})
	// load caches for mutable values to avoid race condition
	svc.store.GetBlockEpochState()
	svc.store.GetHighestLamport()

	svc.emitter = svc.makeEmitter(signer)

	svc.verWatcher = verwatcher.New(config.VersionWatcher, verwatcher.NewStore(store.table.NetworkVersion))

	return svc, nil
}

// makeCheckers builds event checkers
func makeCheckers(heavyCheckCfg heavycheck.Config, chainID *big.Int, heavyCheckReader *HeavyCheckReader, gasPowerCheckReader *GasPowerCheckReader, store *Store) *eventcheck.Checkers {
	// create signatures checker
	heavyCheck := heavycheck.New(heavyCheckCfg, heavyCheckReader, types.NewEIP155Signer(chainID))

	// create gaspower checker
	gaspowerCheck := gaspowercheck.New(gasPowerCheckReader)

	return &eventcheck.Checkers{
		Basiccheck:    basiccheck.New(),
		Epochcheck:    epochcheck.New(store),
		Parentscheck:  parentscheck.New(),
		Heavycheck:    heavyCheck,
		Gaspowercheck: gaspowerCheck,
	}
}

func (s *Service) makeEmitter(signer valkeystore.SignerI) *emitter.Emitter {
	txSigner := types.NewEIP155Signer(s.store.GetRules().EvmChainConfig().ChainID)

	return emitter.NewEmitter(s.config.Emitter, emitter.World{
		External: &emitterWorld{
			s:       s,
			Store:   s.store,
			WgMutex: wgmutex.New(s.engineMu, &s.blockProcWg),
		},
		TxPool:   s.txpool,
		Signer:   signer,
		TxSigner: txSigner,
	})
}

// Protocols returns protocols the service can communicate on.
func (s *Service) Protocols() []p2p.Protocol {
	protos := make([]p2p.Protocol, len(ProtocolVersions))
	for i, vsn := range ProtocolVersions {
		protos[i] = s.pm.makeProtocol(vsn)
		protos[i].Attributes = []enr.Entry{s.currentEnr()}
	}
	return protos
}

// APIs returns api methods the service wants to expose on rpc channels.
func (s *Service) APIs() []rpc.API {
	apis := ethapi.GetAPIs(s.EthAPI)

	apis = append(apis, []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicEthereumAPI(s),
			Public:    true,
		}, {
			Namespace: "eth",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.EthAPI),
			Public:    true,
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)

	return apis
}

// Start method invoked when the node is ready to start the service.
func (s *Service) Start() error {
	genesis := *s.store.GetGenesisHash()
	s.Topic = discv5.Topic("opera@" + genesis.Hex())

	if s.p2pServer.DiscV5 != nil {
		go func(topic discv5.Topic) {
			s.Log.Info("Starting topic registration")
			defer s.Log.Info("Terminated topic registration")

			s.p2pServer.DiscV5.RegisterTopic(topic, s.done)
		}(s.Topic)
	}

	s.blockProcTasks.Start(1)

	s.pm.Start(s.p2pServer.MaxPeers)

	s.serverPool.start(s.p2pServer, s.Topic)

	s.emitter.Start()

	s.verWatcher.Start()

	return nil
}

// WaitBlockEnd waits until parallel block processing is complete (if any)
func (s *Service) WaitBlockEnd() {
	s.blockProcWg.Wait()
}

// Stop method invoked when the node terminates the service.
func (s *Service) Stop() error {
	defer log.Info("Fantom service stopped")
	s.verWatcher.Stop()
	close(s.done)
	s.emitter.Stop()
	s.pm.Stop()
	s.wg.Wait()
	s.feed.scope.Close()

	// flush the state at exit, after all the routines stopped
	s.engineMu.Lock()
	defer s.engineMu.Unlock()
	s.stopped = true

	s.blockProcWg.Wait()
	close(s.blockProcTasksDone)
	return s.store.Commit()
}

// AccountManager return node's account manager
func (s *Service) AccountManager() *accounts.Manager {
	return s.accountManager
}
