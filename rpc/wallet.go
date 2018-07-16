// Copyright (c) 2018 Aidos Developer

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package rpc

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"os/exec"
	"sort"
	"strings"

	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/consensus"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/node"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/dgraph-io/badger"

	shellwords "github.com/mattn/go-shellwords"
)

const walletVersion = 1

//Address is an address with its index in HD wallet.
type Address struct {
	EncAddress []byte `json:"encoded_address"`
	address    *address.Address
}

type account struct {
	Index         uint32              `json:"index"`
	CountAddress2 uint32              `json:"count_accress2"`
	Address       map[string]struct{} `json:"address"`
}

type twallet struct {
	Secret struct {
		seed    []byte
		EncSeed []byte `json:"seed"`
		pwd     []byte
	}
	Accounts map[string]*account `json:"accounts"`
	Pool     struct {
		Index   uint32   `json:"index"`
		Address []string `json:"address"`
	} `json:"pool"`
}

var wallet = twallet{
	Accounts: make(map[string]*account),
}

const poolSize = 100

//Init initialize wallet struct.
func Init(s *setting.Setting) error {
	return s.DB.View(func(txn *badger.Txn) error {
		err := db.Get(txn, nil, &wallet, db.HeaderWallet)
		if err != nil && err != badger.ErrKeyNotFound {
			return err
		}
		return nil
	})
}

//locked by mutex
func putHistory(s *setting.Setting, hist []*history) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, nil, hist, db.HeaderWalletHistory)
	})
}

//locked by mutex
func getHistory(s *setting.Setting) ([]*history, error) {
	var hist []*history
	return hist, s.DB.View(func(txn *badger.Txn) error {
		err := db.Get(txn, nil, &hist, db.HeaderWalletHistory)
		if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	})
}

//locked by mutex
func putWallet(s *setting.Setting) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, nil, &wallet, db.HeaderWallet)
	})
}

//locked by mutex
func putAddress(s *setting.Setting, adr *Address, update bool) error {
	if wallet.Secret.pwd == nil {
		return errors.New("need to call walletpassphrase to encrypt address")
	}
	if update {
		adr.EncAddress = encrypt(arypack.Marshal(adr.address), wallet.Secret.pwd)
	} else {
		dat, err2 := decrypt(adr.EncAddress, wallet.Secret.pwd)
		if err2 != nil {
			return err2
		}
		if err := arypack.Unmarshal(dat, &adr.address); err != nil {
			return err
		}
	}
	name := adr.address.Address58()
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, []byte(name), adr, db.HeaderWalletAddress)
	})
}

//locked by mutex
func getAddress(s *setting.Setting, name string) (*Address, error) {
	if wallet.Secret.pwd == nil {
		return nil, errors.New("need to call walletpassphrase to encrypt address")
	}
	var adr Address
	return &adr, s.DB.View(func(txn *badger.Txn) error {
		if err := db.Get(txn, []byte(name), &adr, db.HeaderWalletAddress); err != nil {
			return err
		}
		dat, err2 := decrypt(adr.EncAddress, wallet.Secret.pwd)
		if err2 != nil {
			return err2
		}
		return arypack.Unmarshal(dat, &adr.address)
	})
}

//locked by mutex
func getAllAddress(s *setting.Setting) (map[string]*Address, error) {
	adrs := make(map[string]*Address)
	err := s.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte{byte(db.HeaderWalletAddress)}
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			v, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			var adr Address
			if err := arypack.Unmarshal(v, &adr); err != nil {
				return err
			}
			adrs[string(item.Key()[1:])] = &adr
		}
		return nil
	})
	return adrs, err
}

//IsSecretEmpty returns true if secret is empty.
func IsSecretEmpty(s *setting.Setting) bool {
	return wallet.Secret.EncSeed == nil
}

//InitSecret initialize secret seed.
func InitSecret(s *setting.Setting, pwd []byte) error {
	if wallet.Secret.EncSeed != nil {
		return nil
	}
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		panic(err)
	}
	wallet.Secret.EncSeed = encrypt(seed, pwd)
	return putWallet(s)
}

func clearSecret() {
	wallet.Secret.seed = nil
	wallet.Secret.pwd = nil
}

func decryptSecret(s *setting.Setting, pwd []byte) error {
	var err error
	wallet.Secret.pwd = pwd
	wallet.Secret.seed, err = decrypt(wallet.Secret.EncSeed, pwd)
	if err != nil {
		return err
	}
	return fillPool(s)
}

func fillPool(s *setting.Setting) error {
	if wallet.Secret.seed == nil || wallet.Secret.pwd == nil {
		return nil
	}
	for i := len(wallet.Pool.Address); i < poolSize; i++ {
		seed := address.HDseed(wallet.Secret.seed, address.Height10, 0, wallet.Pool.Index)
		a, err := address.New(address.Height10, seed, s.Config)
		if err != nil {
			return err
		}
		adr := &Address{
			address: a,
		}
		if err := putAddress(s, adr, true); err != nil {
			return err
		}
		wallet.Pool.Address = append(wallet.Pool.Address, a.Address58())
		wallet.Pool.Index++
	}
	return putWallet(s)
}

type utxo struct {
	address     *Address
	addressName string
	*imesh.InoutHash
	value uint64
}

func updateLeaf(s *setting.Setting, sig *address.Signature, adr *Address, adrname string) error {
	signo, err := sig.Index()
	if err != nil {
		return err
	}
	sigadr, err := sig.Address(s.Config)
	if err != nil {
		return err
	}
	log.Println(signo, address.To58(sigadr))

	if address.To58(sigadr) == adrname && adr.address.LeafNo() <= uint64(signo) {
		if err := adr.address.SetLeafNo(uint64(signo) + 1); err != nil {
			return err
		}
	}
	return nil
}
func getAllUTXOs(s *setting.Setting, checkSig bool) ([]*utxo, uint64, error) {
	var utxos []*utxo
	var bals uint64
	for ac := range wallet.Accounts {
		u, bal, err := getUTXO(s, ac, checkSig)
		if err != nil {
			return nil, 0, err
		}
		utxos = append(utxos, u...)
		bals += bal
	}
	return utxos, bals, nil
}
func getUTXO(s *setting.Setting, acname string, checkSig bool) ([]*utxo, uint64, error) {
	var bal uint64
	var utxos []*utxo
	a, ok := wallet.Accounts[acname]
	if !ok {
		return nil, 0, errors.New("account not found")
	}
	var err error
	for adrname := range a.Address {
		var adr *Address
		if checkSig {
			adr, err = getAddress(s, adrname)
			if err != nil {
				return nil, 0, err
			}
		}
		hs, err := imesh.GetHisoty(s, adrname, true)
		if err != nil {
			return nil, 0, err
		}
		for _, h := range hs {
			switch h.Type {
			case imesh.TypeIn:
				fallthrough
			case imesh.TypeMulin:
				fallthrough
			case imesh.TypeTicketin:
				if !checkSig {
					continue
				}
				tr, err := imesh.GetTx(s, h.Hash)
				if err != nil {
					return nil, 0, err
				}
				for _, sig := range tr.Signatures {
					if err := updateLeaf(s, sig, adr, adrname); err != nil {
						return nil, 0, err
					}
				}
			case imesh.TypeOut:
				tr, err := imesh.GetTxInfo(s, h.Hash)
				if err != nil {
					return nil, 0, err
				}
				if tr.Status != imesh.StatusConfirmed {
					continue
				}
				u := &utxo{
					address:     adr,
					addressName: adrname,
					value:       tr.Body.Outputs[h.Index].Value,
					InoutHash:   h,
				}
				utxos = append(utxos, u)
				bal += u.value
			}
		}

		if checkSig {
			if adr == nil {
				log.Fatal("invalid adr")
			}
			if err := putAddress(s, adr, true); err != nil {
				return nil, 0, err
			}
		}
	}
	return utxos, bal, nil
}

func (adr *Address) sign(s *setting.Setting, tr *tx.Transaction) error {
	if adr.address == nil {
		return errors.New("call walletpassphrase first")
	}
	if err := tr.Sign(adr.address); err != nil {
		return err
	}
	return putAddress(s, adr, true)
}

func newAddress10(s *setting.Setting, aname string) (string, error) {
	ac, ok := wallet.Accounts[aname]
	if !ok {
		ac = &account{
			Index:   uint32(len(wallet.Accounts)),
			Address: make(map[string]struct{}),
		}
		wallet.Accounts[aname] = ac
	}
	if len(wallet.Pool.Address) == 0 {
		return "", errors.New("pool is empty")
	}
	a := wallet.Pool.Address[0]
	wallet.Pool.Address = wallet.Pool.Address[1:]
	ac.Address[a] = struct{}{}
	return a, putWallet(s)
}

func newAddress2(s *setting.Setting, ac *account) (*address.Address, error) {
	if wallet.Secret.pwd == nil {
		return nil, errors.New("call walletpassphrase first")
	}
	seed := address.HDseed(wallet.Secret.seed, address.Height2, ac.Index, ac.CountAddress2)
	a, err := address.New(address.Height2, seed, s.Config)
	if err != nil {
		return nil, err
	}
	adr := &Address{
		address: a,
	}
	if err := putAddress(s, adr, true); err != nil {
		return nil, err
	}
	ac.Address[a.Address58()] = struct{}{}
	ac.CountAddress2++
	return a, putWallet(s)
}

func findAddress(adrstr string) (string, bool) {
	for name, acc := range wallet.Accounts {
		if _, isMine := acc.Address[adrstr]; isMine {
			return name, true
		}
	}
	return "", false
}

type history struct {
	Account string `json:"account"`
	imesh.InoutHash
}

//GoNotify runs gorouitine to get history of addresses in wallet.
//This func needs to run even if RPC is stopped for collecting history.
func GoNotify(s *setting.Setting, creg func(chan []*imesh.HashWithType)) {
	nnotify := make(chan []*imesh.HashWithType, 10)
	node.RegisterTxNotifier(nnotify)
	cnotify := make(chan []*imesh.HashWithType, 10)

	if s.WalletNotify != "" {
		if creg == nil {
			consensus.RegisterTxNotifier(cnotify)
		} else {
			creg(cnotify)
		}
		go func() {
			for noti := range cnotify {
				if err := walletnotifyRunCommand(s, noti); err != nil {
					log.Println(err)
				}
			}
		}()
	}
	go func() {
		for noti := range nnotify {
			trs := make([]*imesh.TxInfo, 0, len(noti))
			for _, h := range noti {
				if h.Type != tx.TypeNormal {
					continue
				}
				tr, err := imesh.GetTxInfo(s, h.Hash)
				if err != nil {
					log.Println(err)
				}
				trs = append(trs, tr)
			}
			sort.Slice(trs, func(i, j int) bool {
				return trs[i].Received.Before(trs[j].Received)
			})
			if err := walletnotifyUpdate(s, trs); err != nil {
				log.Println(err)
			}
			if s.WalletNotify != "" {
				cnotify <- noti
			}
		}
	}()
}

var debugNotify chan string

func walletnotifyRunCommand(s *setting.Setting, noti []*imesh.HashWithType) error {
start:
	for _, h := range noti {
		if h.Type != tx.TypeNormal {
			continue
		}
		tr, err := imesh.GetTxInfo(s, h.Hash)
		if err != nil {
			return err
		}
		for _, out := range tr.Body.Outputs {
			log.Println(out.Address)
			_, ok := findAddress(out.Address.String())
			if !ok {
				continue
			}
			str, err := runCommand(s, h.Hash)
			if err != nil {
				return err
			}
			if debugNotify != nil {
				debugNotify <- str
			}
			continue start
		}
		for _, in := range tr.Body.Inputs {
			out, err := imesh.PreviousOutput(s, in)
			if err != nil {
				return err
			}
			log.Println(out.Address)
			_, ok := findAddress(out.Address.String())
			if !ok {
				continue
			}
			str, err := runCommand(s, h.Hash)
			if err != nil {
				return err
			}
			if debugNotify != nil {
				debugNotify <- str
			}
			continue start
		}
		log.Println("didn't run cmd", h.Hash)
	}
	return nil
}
func walletnotifyUpdate(s *setting.Setting, trs []*imesh.TxInfo) error {
	mutex.Lock()
	defer mutex.Unlock()
	hist, err := getHistory(s)
	if err != nil {
		return err
	}
	for _, tr := range trs {
		for i, out := range tr.Body.Outputs {
			ac, ok := findAddress(out.Address.String())
			if !ok {
				continue
			}
			hist = append(hist, &history{
				Account: ac,
				InoutHash: imesh.InoutHash{
					Type:  imesh.TypeOut,
					Hash:  tr.Hash,
					Index: byte(i),
				},
			})
		}
		for i, in := range tr.Body.Inputs {
			out, err := imesh.PreviousOutput(s, in)
			if err != nil {
				return err
			}
			ac, ok := findAddress(out.Address.String())
			if !ok {
				continue
			}
			hist = append(hist, &history{
				Account: ac,
				InoutHash: imesh.InoutHash{
					Type:  imesh.TypeIn,
					Hash:  tr.Hash,
					Index: byte(i),
				},
			})
		}
		if err := putHistory(s, hist); err != nil {
			return err
		}
	}
	return nil
}

func runCommand(conf *setting.Setting, h tx.Hash) (string, error) {
	if conf.WalletNotify == "" {
		return "", nil
	}
	cmd := strings.Replace(conf.WalletNotify, "%s", hex.EncodeToString(h), -1)
	args, err := shellwords.Parse(cmd)
	if err != nil {
		return "", err
	}
	var out []byte
	if len(args) == 1 {
		out, err = exec.Command(args[0]).Output()
	} else {
		out, err = exec.Command(args[0], args[1:]...).Output()
	}
	if err != nil {
		return "", err
	}
	log.Println("executed ", cmd, ",output:", string(out))
	return string(out), nil
}
