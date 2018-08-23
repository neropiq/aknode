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
	conf       *setting.Setting
	EncAddress []byte `json:"encoded_address"`
	address    *address.Address
}

//Account represents an account with addresses.
type Account struct {
	Index     uint32              `json:"index"`
	Address2  map[string]struct{} `json:"address2"`
	Address10 map[string]struct{} `json:"address10"`
}

func (ac *Account) allAddress() []string {
	adrs := make([]string, 0, len(ac.Address2)*len(ac.Address10))
	for a := range ac.Address2 {
		adrs = append(adrs, a)
	}
	for a := range ac.Address10 {
		adrs = append(adrs, a)
	}
	return adrs
}

//Wallet represents a wallet in RPC..
type Wallet struct {
	conf   *setting.Setting
	Secret struct {
		seed    []byte
		EncSeed []byte `json:"seed"`
		pwd     []byte
	}
	Accounts map[string]*Account `json:"accounts"`
	Pool     struct {
		Index   uint32   `json:"index"`
		Address []string `json:"address"`
	} `json:"pool"`
}

var wallet = Wallet{
	Accounts: make(map[string]*Account),
}

const poolSize = 20 //FIXME

//Init initialize wallet struct.
func Init(s *setting.Setting) error {
	err := s.DB.View(func(txn *badger.Txn) error {
		err := db.Get(txn, nil, &wallet, db.HeaderWallet)
		if err != nil && err != badger.ErrKeyNotFound {
			return err
		}
		return nil
	})
	wallet.conf = s
	return err

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

//locked by mutex
func putAddress(s *setting.Setting, adr *Address, update bool) error {
	if wallet.Secret.pwd == nil {
		return errors.New("need to call walletpassphrase to encrypt address")
	}
	if update {
		adr.EncAddress = address.EncryptSeed(arypack.Marshal(adr.address), wallet.Secret.pwd)
	} else {
		dat, err2 := address.DecryptSeed(adr.EncAddress, wallet.Secret.pwd)
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
		seed := address.HDseed(wallet.Secret.seed, address.Height10, 0, wallet.Pool.Index)
		a, err := address.New(address.Height10, seed, s.Config)
		if err != nil {
			return err
		}
		adr := &Address{
			conf:    s,
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

func updateLeaf(s *setting.Setting, sig *address.Signature, adr *Address, adrname string) error {
	signo, err := sig.Index()
	if err != nil {
		return err
	}
	sigadr, err := sig.Address(s.Config)
	if err != nil {
		return err
	}

	if address.To58(sigadr) == adrname && adr.address.LeafNo() <= uint64(signo) {
		if err := adr.address.SetLeafNo(uint64(signo) + 1); err != nil {
			return err
		}
	}
	return nil
}
func getAllUTXOs(s *setting.Setting, checkSig bool) ([]*tx.UTXO, uint64, error) {
	var utxos []*tx.UTXO
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
func getUTXO(s *setting.Setting, acname string, checkSig bool) ([]*tx.UTXO, uint64, error) {
	var bal uint64
	var utxos []*tx.UTXO
	u10, b10, err := getUTXO102(s, acname, true, checkSig)
	if err != nil {
		return nil, 0, err
	}
	bal += b10
	utxos = append(utxos, u10...)
	u2, b2, err := getUTXO102(s, acname, false, checkSig)
	if err != nil {
		return nil, 0, err
	}
	bal += b2
	utxos = append(utxos, u2...)
	return utxos, bal, nil
}

func getUTXO102(s *setting.Setting, acname string, is10 bool, checkSig bool) ([]*tx.UTXO, uint64, error) {
	var bal uint64
	var utxos []*tx.UTXO
	a, ok := wallet.Accounts[acname]
	if !ok {
		return nil, 0, errors.New("account not found")
	}
	adrmap := a.Address10
	if !is10 {
		adrmap = a.Address2
	}
	var err error
	for adrname := range adrmap {
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
			case tx.TypeIn:
				fallthrough
			case tx.TypeMulin:
				fallthrough
			case tx.TypeTicketin:
				if !checkSig {
					continue
				}
				tr, err := imesh.GetTx(s.DB, h.Hash)
				if err != nil {
					return nil, 0, err
				}
				for _, sig := range tr.Signatures {
					if err := updateLeaf(s, sig, adr, adrname); err != nil {
						return nil, 0, err
					}
				}
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

func (adr *Address) String() string {
	return adr.address.Address58()
}

//Sign signs a tx and puts the state of the adr to DB.
func (adr *Address) Sign(tr *tx.Transaction) error {
	if adr.address == nil {
		return errors.New("call walletpassphrase first")
	}
	if err := tr.Sign(adr.address); err != nil {
		return err
	}
	return putAddress(adr.conf, adr, true)
}

func newAddress10(s *setting.Setting, aname string) (string, error) {
	ac, ok := wallet.Accounts[aname]
	if !ok {
		ac = &Account{
			Index:     uint32(len(wallet.Accounts)),
			Address2:  make(map[string]struct{}),
			Address10: make(map[string]struct{}),
		}
		wallet.Accounts[aname] = ac
	}
	if len(wallet.Pool.Address) == 0 {
		return "", errors.New("pool is empty")
	}
	a := wallet.Pool.Address[0]
	wallet.Pool.Address = wallet.Pool.Address[1:]
	ac.Address10[a] = struct{}{}
	return a, putWallet(s)
}

//NewAddress2 returns a new address with height=2.
func (w *Wallet) NewAddress2(acstr string) (*address.Address, error) {
	if acstr == "*" {
		for acstr = range wallet.Accounts {
			break
		}
	}
	ac, ok := wallet.Accounts[acstr]
	if !ok {
		return nil, errors.New("invalid account name")
	}
	if wallet.Secret.pwd == nil {
		return nil, errors.New("call walletpassphrase first")
	}
	seed := address.HDseed(wallet.Secret.seed, address.Height2, ac.Index, uint32(len(ac.Address2)))
	a, err := address.New(address.Height2, seed, w.conf.Config)
	if err != nil {
		return nil, err
	}
	adr := &Address{
		conf:    w.conf,
		address: a,
	}
	if err := putAddress(w.conf, adr, true); err != nil {
		return nil, err
	}
	ac.Address2[a.Address58()] = struct{}{}
	return a, putWallet(w.conf)
}

func findAddress(adrstr string) (string, bool) {
	for name, acc := range wallet.Accounts {
		if _, isMine := acc.Address2[adrstr]; isMine {
			return name, true
		}
		if _, isMine := acc.Address10[adrstr]; isMine {
			return name, true
		}
	}
	return "", false
}

//History is a tx history for an account.
type History struct {
	Account string `json:"account"`
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
			_, ok := findAddress(out.Address.String())
			if !ok {
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
			_, ok := findAddress(out.Address.String())
			if !ok {
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
			ac, ok := findAddress(out.Address.String())
			if !ok {
				continue
			}
			hist = append(hist, &History{
				Account: ac,
				InoutHash: tx.InoutHash{
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
			ac, ok := findAddress(out.Address.String())
			if !ok {
				continue
			}
			hist = append(hist, &History{
				Account: ac,
				InoutHash: tx.InoutHash{
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
