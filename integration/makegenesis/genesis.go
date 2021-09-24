package makegenesis

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/ethereum/go-ethereum/log"
	"github.com/status-im/keycard-go/hexutils"
	"github.com/zilionixx/go-zilionixx/utils"
	"github.com/zilionixx/go-zilionixx/zilionixx"
	"github.com/zilionixx/zilion-base/hash"
	"github.com/zilionixx/zilion-base/inter/idx"

	"github.com/zilionixx/go-zilionixx/inter"
	"github.com/zilionixx/go-zilionixx/inter/validatorpk"
	"github.com/zilionixx/go-zilionixx/zilionixx/genesis"
	"github.com/zilionixx/go-zilionixx/zilionixx/genesis/driver"
	"github.com/zilionixx/go-zilionixx/zilionixx/genesis/driverauth"
	"github.com/zilionixx/go-zilionixx/zilionixx/genesis/evmwriter"
	"github.com/zilionixx/go-zilionixx/zilionixx/genesis/gpos"
	"github.com/zilionixx/go-zilionixx/zilionixx/genesis/netinit"
	"github.com/zilionixx/go-zilionixx/zilionixx/genesis/sfc"
	"github.com/zilionixx/go-zilionixx/zilionixx/genesisstore"
)

var (
	FakeGenesisTime = inter.Timestamp(1608600000 * time.Second)
	TestNetGenesisTime = inter.Timestamp(1608600000 * time.Second)
	MainNetNetGenesisTime = inter.Timestamp(1608600000 * time.Second)
)

// FakeKey gets n-th fake private key.
func FakeKey(n int) *ecdsa.PrivateKey {
	reader := rand.New(rand.NewSource(int64(n)))

	key, err := ecdsa.GenerateKey(crypto.S256(), reader)
	if err != nil {
		panic(err)
	}

	return key
}

func FakeGenesisStore(num int, balance, stake *big.Int) *genesisstore.Store {
	genStore := genesisstore.NewMemStore()
	genStore.SetRules(zilionixx.FakeNetRules())

	validators := GetFakeValidators(num)

	totalSupply := new(big.Int)
	for _, val := range validators {
		genStore.SetEvmAccount(val.Address, genesis.Account{
			Code:    []byte{},
			Balance: balance,
			Nonce:   0,
		})
		genStore.SetDelegation(val.Address, val.ID, genesis.Delegation{
			Stake:              stake,
			Rewards:            new(big.Int),
			LockedStake:        new(big.Int),
			LockupFromEpoch:    0,
			LockupEndTime:      0,
			LockupDuration:     0,
			EarlyUnlockPenalty: new(big.Int),
		})
		totalSupply.Add(totalSupply, balance)
	}

	var owner common.Address
	if num != 0 {
		owner = validators[0].Address
	}

	genStore.SetMetadata(genesisstore.Metadata{
		Validators:    validators,
		FirstEpoch:    2,
		Time:          FakeGenesisTime,
		PrevEpochTime: FakeGenesisTime - inter.Timestamp(time.Hour),
		ExtraData:     []byte("fake"),
		DriverOwner:   owner,
		TotalSupply:   totalSupply,
	})
	genStore.SetBlock(0, genesis.Block{
		Time:        FakeGenesisTime - inter.Timestamp(time.Minute),
		Atropos:     hash.Event{},
		Txs:         types.Transactions{},
		InternalTxs: types.Transactions{},
		Root:        hash.Hash{},
		Receipts:    []*types.ReceiptForStorage{},
	})
	// pre deploy NetworkInitializer
	genStore.SetEvmAccount(netinit.ContractAddress, genesis.Account{
		Code:    netinit.GetContractBin(),
		Balance: new(big.Int),
		Nonce:   0,
	})
	// pre deploy NodeDriver
	genStore.SetEvmAccount(driver.ContractAddress, genesis.Account{
		Code:    driver.GetContractBin(),
		Balance: new(big.Int),
		Nonce:   0,
	})
	// pre deploy NodeDriverAuth
	genStore.SetEvmAccount(driverauth.ContractAddress, genesis.Account{
		Code:    driverauth.GetContractBin(),
		Balance: new(big.Int),
		Nonce:   0,
	})
	// pre deploy SFC
	genStore.SetEvmAccount(sfc.ContractAddress, genesis.Account{
		Code:    sfc.GetContractBin(),
		Balance: new(big.Int),
		Nonce:   0,
	})
	// set non-zero code for pre-compiled contracts
	genStore.SetEvmAccount(evmwriter.ContractAddress, genesis.Account{
		Code:    []byte{0},
		Balance: new(big.Int),
		Nonce:   0,
	})

	return genStore
}

func GetFakeValidators(num int) gpos.Validators {
	validators := make(gpos.Validators, 0, num)

	for i := 1; i <= num; i++ {
		key := FakeKey(i)
		addr := crypto.PubkeyToAddress(key.PublicKey)
		pubkeyraw := crypto.FromECDSAPub(&key.PublicKey)
		validatorID := idx.ValidatorID(i)
		validators = append(validators, gpos.Validator{
			ID:      validatorID,
			Address: addr,
			PubKey: validatorpk.PubKey{
				Raw:  pubkeyraw,
				Type: validatorpk.Types.Secp256k1,
			},
			CreationTime:     FakeGenesisTime,
			CreationEpoch:    0,
			DeactivatedTime:  0,
			DeactivatedEpoch: 0,
			Status:           0,
		})
	}

	return validators
}

func GetTestNetValidators() gpos.Validators {
	validators := make(gpos.Validators, 0, 3)

	key, _ := GetPrivateKey(hexutils.HexToBytes("FFFF9C7034E0EB6AD8D496DDAD05004EE4A5C5195C4A08801E0C589F5CFBDBB5"), secp256k1.S256())
	addr := crypto.PubkeyToAddress(key.PublicKey)
	pubkeyraw := crypto.FromECDSAPub(&key.PublicKey)
	validatorID := idx.ValidatorID(1)
	log.Debug("***********", "pubkeyraw", pubkeyraw)
	validators = append(validators, gpos.Validator{
		ID:      validatorID,
		Address: addr,
		PubKey: validatorpk.PubKey{
			Raw:  pubkeyraw,
			Type: validatorpk.Types.Secp256k1,
		},
		CreationTime:     TestNetGenesisTime,
		CreationEpoch:    0,
		DeactivatedTime:  0,
		DeactivatedEpoch: 0,
		Status:           0,
	})

	key, _ = GetPrivateKey(hexutils.HexToBytes("13F91D84E1C4B594A180959F68A1CC129C283FBC60277FDA68A4A443D2D0B1BF"), secp256k1.S256())
	addr = crypto.PubkeyToAddress(key.PublicKey)
	pubkeyraw = crypto.FromECDSAPub(&key.PublicKey)
	validatorID = idx.ValidatorID(2)
	log.Debug("***********", "pubkeyraw", pubkeyraw)
	validators = append(validators, gpos.Validator{
		ID:      validatorID,
		Address: addr,
		PubKey: validatorpk.PubKey{
			Raw:  pubkeyraw,
			Type: validatorpk.Types.Secp256k1,
		},
		CreationTime:     TestNetGenesisTime,
		CreationEpoch:    0,
		DeactivatedTime:  0,
		DeactivatedEpoch: 0,
		Status:           0,
	})

	key, _ = GetPrivateKey(hexutils.HexToBytes("68DFF6ECD0BA01DD6DD08B8FFD66DFEA8E647A7530B3ABE4B1EF609B00E18D36"), secp256k1.S256())
	addr = crypto.PubkeyToAddress(key.PublicKey)
	pubkeyraw = crypto.FromECDSAPub(&key.PublicKey)
	validatorID = idx.ValidatorID(3)
	log.Debug("***********", "pubkeyraw", pubkeyraw)
	validators = append(validators, gpos.Validator{
		ID:      validatorID,
		Address: addr,
		PubKey: validatorpk.PubKey{
			Raw:  pubkeyraw,
			Type: validatorpk.Types.Secp256k1,
		},
		CreationTime:     TestNetGenesisTime,
		CreationEpoch:    0,
		DeactivatedTime:  0,
		DeactivatedEpoch: 0,
		Status:           0,
	})
	return validators
}

func GetMainNetValidators() gpos.Validators {
	validators := make(gpos.Validators, 0, 3)

	key, _ := GetPrivateKey(hexutils.HexToBytes("FFFF9C7034E0EB6AD8D496DDAD05004EE4A5C5195C4A08801E0C589F5CFBDBB5"), secp256k1.S256())
	addr := crypto.PubkeyToAddress(key.PublicKey)
	pubkeyraw := crypto.FromECDSAPub(&key.PublicKey)
	validatorID := idx.ValidatorID(1)
	log.Debug("***********", "pubkeyraw", pubkeyraw)
	validators = append(validators, gpos.Validator{
		ID:      validatorID,
		Address: addr,
		PubKey: validatorpk.PubKey{
			Raw:  pubkeyraw,
			Type: validatorpk.Types.Secp256k1,
		},
		CreationTime:     TestNetGenesisTime,
		CreationEpoch:    0,
		DeactivatedTime:  0,
		DeactivatedEpoch: 0,
		Status:           0,
	})

	key, _ = GetPrivateKey(hexutils.HexToBytes("13F91D84E1C4B594A180959F68A1CC129C283FBC60277FDA68A4A443D2D0B1BF"), secp256k1.S256())
	addr = crypto.PubkeyToAddress(key.PublicKey)
	pubkeyraw = crypto.FromECDSAPub(&key.PublicKey)
	validatorID = idx.ValidatorID(2)
	log.Debug("***********", "pubkeyraw", pubkeyraw)
	validators = append(validators, gpos.Validator{
		ID:      validatorID,
		Address: addr,
		PubKey: validatorpk.PubKey{
			Raw:  pubkeyraw,
			Type: validatorpk.Types.Secp256k1,
		},
		CreationTime:     TestNetGenesisTime,
		CreationEpoch:    0,
		DeactivatedTime:  0,
		DeactivatedEpoch: 0,
		Status:           0,
	})

	key, _ = GetPrivateKey(hexutils.HexToBytes("68DFF6ECD0BA01DD6DD08B8FFD66DFEA8E647A7530B3ABE4B1EF609B00E18D36"), secp256k1.S256())
	addr = crypto.PubkeyToAddress(key.PublicKey)
	pubkeyraw = crypto.FromECDSAPub(&key.PublicKey)
	validatorID = idx.ValidatorID(3)
	log.Debug("***********", "pubkeyraw", pubkeyraw)
	validators = append(validators, gpos.Validator{
		ID:      validatorID,
		Address: addr,
		PubKey: validatorpk.PubKey{
			Raw:  pubkeyraw,
			Type: validatorpk.Types.Secp256k1,
		},
		CreationTime:     TestNetGenesisTime,
		CreationEpoch:    0,
		DeactivatedTime:  0,
		DeactivatedEpoch: 0,
		Status:           0,
	})
	return validators
}

// GetPrivateKey generates a public and private key pair.
func GetPrivateKey(privkey []byte, c elliptic.Curve) (*ecdsa.PrivateKey, error) {
	k := new(big.Int)

	k.SetBytes(privkey)
	key := new(ecdsa.PrivateKey)
	key.PublicKey.Curve = secp256k1.S256()
	key.D = k
	key.PublicKey.X, key.PublicKey.Y = secp256k1.S256().ScalarBaseMult(k.Bytes())

	return key, nil
}

func TestNetGenesisStore(balance, stake *big.Int) *genesisstore.Store {
	genStore := genesisstore.NewMemStore()
	genStore.SetRules(zilionixx.TestNetRules())

	validators := GetTestNetValidators()

	totalSupply := new(big.Int)
	for _, val := range validators {
		genStore.SetEvmAccount(val.Address, genesis.Account{
			Code:    []byte{},
			Balance: balance,
			Nonce:   0,
		})
		genStore.SetDelegation(val.Address, val.ID, genesis.Delegation{
			Stake:              stake,
			Rewards:            new(big.Int),
			LockedStake:        new(big.Int),
			LockupFromEpoch:    0,
			LockupEndTime:      0,
			LockupDuration:     0,
			EarlyUnlockPenalty: new(big.Int),
		})
		totalSupply.Add(totalSupply, balance)
	}

	genStore.SetEvmAccount(common.HexToAddress("0xF7C913733Ab38Aa2b3a4a69B2Bf640d5E340Fc43"), genesis.Account{
		Code:    []byte{},
		Balance: utils.ToFtm(88888888),
		Nonce:   0,
	})

	totalSupply.Add(totalSupply, utils.ToFtm(88888888))

	owner := validators[0].Address

	genStore.SetMetadata(genesisstore.Metadata{
		Validators:    validators,
		FirstEpoch:    2,
		Time:          TestNetGenesisTime,
		PrevEpochTime: TestNetGenesisTime - inter.Timestamp(time.Hour),
		ExtraData:     []byte("testnet"),
		DriverOwner:   owner,
		TotalSupply:   totalSupply,
	})
	genStore.SetBlock(0, genesis.Block{
		Time:        TestNetGenesisTime - inter.Timestamp(time.Minute),
		Atropos:     hash.Event{},
		Txs:         types.Transactions{},
		InternalTxs: types.Transactions{},
		Root:        hash.Hash{},
		Receipts:    []*types.ReceiptForStorage{},
	})
	// pre deploy NetworkInitializer
	genStore.SetEvmAccount(netinit.ContractAddress, genesis.Account{
		Code:    netinit.GetContractBin(),
		Balance: new(big.Int),
		Nonce:   0,
	})
	// pre deploy NodeDriver
	genStore.SetEvmAccount(driver.ContractAddress, genesis.Account{
		Code:    driver.GetContractBin(),
		Balance: new(big.Int),
		Nonce:   0,
	})
	// pre deploy NodeDriverAuth
	genStore.SetEvmAccount(driverauth.ContractAddress, genesis.Account{
		Code:    driverauth.GetContractBin(),
		Balance: new(big.Int),
		Nonce:   0,
	})
	// pre deploy SFC
	genStore.SetEvmAccount(sfc.ContractAddress, genesis.Account{
		Code:    sfc.GetContractBin(),
		Balance: new(big.Int),
		Nonce:   0,
	})
	// set non-zero code for pre-compiled contracts
	genStore.SetEvmAccount(evmwriter.ContractAddress, genesis.Account{
		Code:    []byte{0},
		Balance: new(big.Int),
		Nonce:   0,
	})

	return genStore
}

func MainNetGenesisStore(balance, stake *big.Int) *genesisstore.Store {
	genStore := genesisstore.NewMemStore()
	genStore.SetRules(zilionixx.MainNetRules())

	validators := GetMainNetValidators()

	totalSupply := new(big.Int)
	for _, val := range validators {
		genStore.SetEvmAccount(val.Address, genesis.Account{
			Code:    []byte{},
			Balance: balance,
			Nonce:   0,
		})
		genStore.SetDelegation(val.Address, val.ID, genesis.Delegation{
			Stake:              stake,
			Rewards:            new(big.Int),
			LockedStake:        new(big.Int),
			LockupFromEpoch:    0,
			LockupEndTime:      0,
			LockupDuration:     0,
			EarlyUnlockPenalty: new(big.Int),
		})
		totalSupply.Add(totalSupply, balance)
	}

	genStore.SetEvmAccount(common.HexToAddress("0xF7C913733Ab38Aa2b3a4a69B2Bf640d5E340Fc43"), genesis.Account{
		Code:    []byte{},
		Balance: utils.ToFtm(88888888),
		Nonce:   0,
	})

	totalSupply.Add(totalSupply, utils.ToFtm(88888888))

	owner := validators[0].Address

	genStore.SetMetadata(genesisstore.Metadata{
		Validators:    validators,
		FirstEpoch:    2,
		Time:          MainNetNetGenesisTime,
		PrevEpochTime: MainNetNetGenesisTime - inter.Timestamp(time.Hour),
		ExtraData:     []byte("testnet"),
		DriverOwner:   owner,
		TotalSupply:   totalSupply,
	})
	genStore.SetBlock(0, genesis.Block{
		Time:        MainNetNetGenesisTime - inter.Timestamp(time.Minute),
		Atropos:     hash.Event{},
		Txs:         types.Transactions{},
		InternalTxs: types.Transactions{},
		Root:        hash.Hash{},
		Receipts:    []*types.ReceiptForStorage{},
	})
	// pre deploy NetworkInitializer
	genStore.SetEvmAccount(netinit.ContractAddress, genesis.Account{
		Code:    netinit.GetContractBin(),
		Balance: new(big.Int),
		Nonce:   0,
	})
	// pre deploy NodeDriver
	genStore.SetEvmAccount(driver.ContractAddress, genesis.Account{
		Code:    driver.GetContractBin(),
		Balance: new(big.Int),
		Nonce:   0,
	})
	// pre deploy NodeDriverAuth
	genStore.SetEvmAccount(driverauth.ContractAddress, genesis.Account{
		Code:    driverauth.GetContractBin(),
		Balance: new(big.Int),
		Nonce:   0,
	})
	// pre deploy SFC
	genStore.SetEvmAccount(sfc.ContractAddress, genesis.Account{
		Code:    sfc.GetContractBin(),
		Balance: new(big.Int),
		Nonce:   0,
	})
	// set non-zero code for pre-compiled contracts
	genStore.SetEvmAccount(evmwriter.ContractAddress, genesis.Account{
		Code:    []byte{0},
		Balance: new(big.Int),
		Nonce:   0,
	})

	return genStore
}
