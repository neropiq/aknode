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
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/dgraph-io/badger"

	shellwords "github.com/mattn/go-shellwords"
)

const walletVersion = 1

//Address is an address with its index in HD wallet.
type Address struct {
	EncAddress []byte `json:"encoded_address"`
	address    *address.Address
	Adrstr     string
}

//Wallet represents a wallet in RPC..
type Wallet struct {
	Secret struct {
		seed    []byte
		EncSeed []byte `json:"seed"`
		pwd     []byte
	}
	AccountName   string
	AddressPublic map[string]struct{} `json:"address_public"`
	AddressChange map[string]struct{} `json:"address_change"`
	Pool          struct {
		Index   uint32   `json:"index"`
		Address []string `json:"address"`
	} `json:"pool"`
}

var wallet = Wallet{
	AddressChange: make(map[string]struct{}),
	AddressPublic: make(map[string]struct{}),
}

const poolSize = 20 //FIXME

//Init initialize wallet struct.
func Init(s *setting.Setting) error {
	wallet = Wallet{
		AddressChange: make(map[string]struct{}),
		AddressPublic: make(map[string]struct{}),
	}
	err := s.DB.View(func(txn *badger.Txn) error {
		err := db.Get(txn, nil, &wallet, db.HeaderWallet)
		if err != nil && err != badger.ErrKeyNotFound {
			return err
		}
		return nil
	})
	return err

}

func allAddress() []string {
	adrs := make([]string, 0, len(wallet.AddressChange)*len(wallet.AddressPublic))
	for a := range wallet.AddressChange {
		adrs = append(adrs, a)
	}
	for a := range wallet.AddressPublic {
		adrs = append(adrs, a)
	}
	return adrs
}

//locked by mutex
func putHistory(s *setting.Setting, hist []*History) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, nil, hist, db.HeaderWalletHistory)
	})
}

//locked by mutex
func getHistory(s *setting.Setting) ([]*History, error) {
	var hist []*History
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

func (adr *Address) encrypt() error {
	if wallet.Secret.pwd == nil {
		return errors.New("need to call walletpassphrase to encrypt address")
	}
	adr.EncAddress = address.EncryptSeed(arypack.Marshal(adr.address), wallet.Secret.pwd)
	return nil
}

//locked by mutex
func putAddress(s *setting.Setting, adr *Address) error {
	if adr.EncAddress == nil {
		return errors.New("empty encaddress")
	}
	if adr.Adrstr == "" {
		return errors.New("empty adrstr")
	}
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, []byte(adr.Adrstr), adr, db.HeaderWalletAddress)
	})
}

//locked by mutex
func getAddress(s *setting.Setting, name string) (*Address, error) {
	log.Println(name)

	var adr Address
	return &adr, s.DB.View(func(txn *badger.Txn) error {
		if err := db.Get(txn, []byte(name), &adr, db.HeaderWalletAddress); err != nil {
			log.Println(err)
			return err
		}
		if wallet.Secret.pwd == nil {
			return nil
		}
		dat, err2 := address.DecryptSeed(adr.EncAddress, wallet.Secret.pwd)
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
func IsSecretEmpty() bool {
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
	wallet.Secret.EncSeed = address.EncryptSeed(seed, pwd)
	wallet.Secret.seed = seed
	wallet.Secret.pwd = pwd
	if err := fillPool(s); err != nil {
		return err
	}
	wallet.Secret.seed = nil
	wallet.Secret.pwd = nil
	return putWallet(s)
}

func clearSecret() {
	wallet.Secret.seed = nil
	wallet.Secret.pwd = nil
}

func decryptSecret(s *setting.Setting, pwd []byte) error {
	var err error
	wallet.Secret.seed, err = address.DecryptSeed(wallet.Secret.EncSeed, pwd)
	if err != nil {
		return err
	}
	wallet.Secret.pwd = pwd
	return fillPool(s)
}

func fillPool(s *setting.Setting) error {
	if wallet.Secret.seed == nil || wallet.Secret.pwd == nil {
		return nil
	}
	for i := len(wallet.Pool.Address); i < poolSize; i++ {
		seed := address.HDseed(wallet.Secret.seed, 0, wallet.Pool.Index)
		a, err := address.New(s.Config, seed)
		if err != nil {
			return err
		}
		adr := &Address{
			address: a,
			Adrstr:  a.Address58(s.Config),
		}
		if err := adr.encrypt(); err != nil {
			return err
		}
		if err := putAddress(s, adr); err != nil {
			return err
		}
		wallet.Pool.Address = append(wallet.Pool.Address, a.Address58(s.Config))
		wallet.Pool.Index++
	}
	return putWallet(s)
}

func getUTXO(s *setting.Setting) ([]*tx.UTXO, uint64, error) {
	var bal uint64
	var utxos []*tx.UTXO
	u10, b10, err := getUTXO102(s, true)
	if err != nil {
		return nil, 0, err
	}
	bal += b10
	utxos = append(utxos, u10...)
	u2, b2, err := getUTXO102(s, false)
	if err != nil {
		return nil, 0, err
	}
	bal += b2
	utxos = append(utxos, u2...)
	return utxos, bal, nil
}

func getUTXO102(s *setting.Setting, isPublic bool) ([]*tx.UTXO, uint64, error) {
	var bal uint64
	var utxos []*tx.UTXO
	adrmap := wallet.AddressChange
	if isPublic {
		adrmap = wallet.AddressPublic
	}
	for adrname := range adrmap {
		var adr *Address
		var err error
		adr, err = getAddress(s, adrname)
		if err != nil {
			return nil, 0, err
		}
		hs, err := imesh.GetHisoty(s, adrname, true)
		if err != nil {
			return nil, 0, err
		}
		for _, h := range hs {
			switch h.Type {
			case tx.TypeOut:
				tr, err := imesh.GetTxInfo(s.DB, h.Hash)
				if err != nil {
					return nil, 0, err
				}
				if tr.Status != imesh.StatusConfirmed {
					continue
				}
				u := &tx.UTXO{
					Address:   adr,
					Value:     tr.Body.Outputs[h.Index].Value,
					InoutHash: h,
				}
				utxos = append(utxos, u)
				bal += u.Value
			}
		}
	}
	return utxos, bal, nil
}

func (adr *Address) String() string {
	return adr.Adrstr
}

//Sign signs a tx and puts the state of the adr to DB.
func (adr *Address) Sign(tr *tx.Transaction) error {
	if adr.address == nil {
		return errors.New("call walletpassphrase first")
	}
	return tr.Sign(adr.address)
}

func newPublicAddress(s *setting.Setting) (string, error) {
	if len(wallet.Pool.Address) == 0 {
		return "", errors.New("pool is empty")
	}
	a := wallet.Pool.Address[0]
	wallet.Pool.Address = wallet.Pool.Address[1:]
	wallet.AddressPublic[a] = struct{}{}
	return a, putWallet(s)
}

func newChangeAddress(s *setting.Setting) (string, error) {
	if wallet.Secret.seed == nil || wallet.Secret.pwd == nil {
		return "", errors.New("call walletpasspharse first")
	}
	seed := address.HDseed(wallet.Secret.seed, 1, uint32(len(wallet.AddressChange)))
	a, err := address.New(s.Config, seed)
	if err != nil {
		return "", err
	}
	adr := &Address{
		address: a,
		Adrstr:  a.Address58(s.Config),
	}
	if err := adr.encrypt(); err != nil {
		return "", err
	}
	if err := putAddress(s, adr); err != nil {
		return "", err
	}
	log.Println(adr, adr.Adrstr)
	wallet.AddressChange[adr.Adrstr] = struct{}{}
	return adr.Adrstr, putWallet(s)
}

func findAddress(adrstr string) bool {
	if _, isMine := wallet.AddressPublic[adrstr]; isMine {
		return true
	}
	if _, isMine := wallet.AddressChange[adrstr]; isMine {
		return true
	}
	return false
}

//History is a tx history for an account.
type History struct {
	tx.InoutHash
}

//GetOutput returns an output related to InOutHash ih.
//If ih is output, returns the output specified by ih.
//If ih is input, return output refered by the input.
func (h *History) GetOutput(s *setting.Setting) (*tx.Output, error) {
	return imesh.GetOutput(s, &h.InoutHash)
}

//GoNotify runs gorouitine to get history of addresses in wallet.
//This func needs to run even if RPC is stopped for collecting history.
func GoNotify(s *setting.Setting, nreg, creg func(chan []tx.Hash)) {
	nnotify := make(chan []tx.Hash, 10)
	nreg(nnotify)
	cnotify := make(chan []tx.Hash, 10)

	if s.WalletNotify != "" {
		creg(cnotify)
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
				tr, err := imesh.GetTxInfo(s.DB, h)
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

func walletnotifyRunCommand(s *setting.Setting, noti []tx.Hash) error {
start:
	for _, h := range noti {
		tr, err := imesh.GetTxInfo(s.DB, h)
		if err != nil {
			return err
		}
		for _, out := range tr.Body.Outputs {
			if !findAddress(out.Address.String()) {
				continue
			}
			str, err := runCommand(s, h)
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
			if !findAddress(out.Address.String()) {
				continue
			}
			str, err := runCommand(s, h)
			if err != nil {
				return err
			}
			if debugNotify != nil {
				debugNotify <- str
			}
			continue start
		}
		log.Println("didn't run cmd", h)
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
			if !findAddress(out.Address.String()) {
				continue
			}
			hist = append(hist, &History{
				tx.InoutHash{
					Type:  tx.TypeOut,
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
			if !findAddress(out.Address.String()) {
				continue
			}
			hist = append(hist, &History{
				tx.InoutHash{
					Type:  tx.TypeIn,
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
