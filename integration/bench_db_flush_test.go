package integration

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/zilionixx/zilion-base/abft"
	"github.com/zilionixx/zilion-base/hash"
	"github.com/zilionixx/zilion-base/inter/idx"
	"github.com/zilionixx/zilion-base/kvdb"
	"github.com/zilionixx/zilion-base/kvdb/leveldb"
	"github.com/zilionixx/zilion-base/utils/cachescale"

	"github.com/zilionixx/go-zilionixx/gossip"
	"github.com/zilionixx/go-zilionixx/integration/makegenesis"
	"github.com/zilionixx/go-zilionixx/inter"
	"github.com/zilionixx/go-zilionixx/utils"
	"github.com/zilionixx/go-zilionixx/vecmt"
	"github.com/zilionixx/go-zilionixx/zilionixx/genesisstore"
)

func BenchmarkFlushDBs(b *testing.B) {
	rawProducer, dir := dbProducer("flush_bench")
	defer os.RemoveAll(dir)
	genStore := makegenesis.FakeGenesisStore(1, utils.ToZnx(1), utils.ToZnx(1))
	_, _, store, s2, s3, _ := MakeEngine(rawProducer, InputGenesis{
		Hash: genStore.Hash(),
		Read: func(store *genesisstore.Store) error {
			buf := bytes.NewBuffer(nil)
			err := genStore.Export(buf)
			if err != nil {
				return err
			}
			return store.Import(buf)
		},
		Close: func() error {
			return nil
		},
	}, Configs{
		Zilionixx:         gossip.DefaultConfig(cachescale.Identity),
		ZilionixxStore:    gossip.DefaultStoreConfig(cachescale.Identity),
		ZilionBFT:      abft.DefaultConfig(),
		ZilionBFTStore: abft.DefaultStoreConfig(cachescale.Identity),
		VectorClock:   vecmt.DefaultConfig(cachescale.Identity),
	})
	defer store.Close()
	defer s2.Close()
	defer s3.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := idx.Block(0)
		randUint32s := func() []uint32 {
			arr := make([]uint32, 128)
			for i := 0; i < len(arr); i++ {
				arr[i] = uint32(i) ^ (uint32(n) << 16) ^ 0xd0ad884e
			}
			return []uint32{uint32(n), uint32(n) + 1, uint32(n) + 2}
		}
		for !store.IsCommitNeeded(false) {
			store.SetBlock(n, &inter.Block{
				Time:        inter.Timestamp(n << 32),
				Atropos:     hash.Event{},
				Events:      hash.Events{},
				Txs:         []common.Hash{},
				InternalTxs: []common.Hash{},
				SkippedTxs:  randUint32s(),
				GasUsed:     uint64(n) << 24,
				Root:        hash.Hash{},
			})
			n++
		}
		err := store.Commit()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func cache64mb(string) int {
	return 64 * opt.MiB
}

func dbProducer(name string) (kvdb.IterableDBProducer, string) {
	dir, err := ioutil.TempDir("", name)
	if err != nil {
		panic(err)
	}
	return leveldb.NewProducer(dir, cache64mb), dir
}
