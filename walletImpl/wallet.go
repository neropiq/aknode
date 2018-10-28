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

package walletImpl

import (
	"crypto/rand"
	"errors"
	"log"
	"time"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"

	"github.com/dgraph-io/badger"
)

//Address is an address with its index in HD wallet.
type Address struct {
	EncAddress []byte           `json:"encoded_address"`
	Address    *address.Address `json:"-" msgpack:"-"`
	Adrstr     string
}

//Pool is a pool for addresses.
type Pool struct {
	Index   uint32   `json:"index"`
	Address []string `json:"address"`
}

//Wallet represents a wallet in RPC..
type Wallet struct {
	AccountName   string              `json:"account_name"`
	EncSeed       []byte              `json:"enc_secret"`
	AddressChange map[string]struct{} `json:"address_public"`
	AddressPublic map[string]struct{} `json:"address_change"`
	Pool          *Pool               `json:"pool"`
}

const poolSize = 20 //FIXME

//FillPool fills the pool.
func (w *Wallet) FillPool(s *aklib.DBConfig, pwdd []byte) error {
	master, err := address.DecryptSeed(w.EncSeed, pwdd)
	if err != nil {
		return err
	}
	for i := len(w.Pool.Address); i < poolSize; i++ {
		seed := address.HDseed(master, 0, w.Pool.Index)
		a, err := address.New(s.Config, seed)
		if err != nil {
			return err
		}
		adr := &Address{
			Address: a,
			Adrstr:  a.Address58(s.Config),
		}
		adr.Encrypt(pwdd)
		if err := w.PutAddress(s, pwdd, adr, true); err != nil {
			return err
		}
		w.Pool.Address = append(w.Pool.Address, a.Address58(s.Config))
		w.Pool.Index++
	}
	return w.put(s)
}

//AllAddress returns all addresses in the wallet.
func (w *Wallet) AllAddress() []string {
	adrs := make([]string, 0, len(w.AddressChange)+len(w.AddressPublic))
	for adr := range w.AddressChange {
		adrs = append(adrs, adr)
	}
	for adr := range w.AddressPublic {
		adrs = append(adrs, adr)
	}
	return adrs
}

//Load initialize wallet struct.
func Load(s *aklib.DBConfig, pwd []byte, priv string) (*Wallet, error) {
	var wallet = Wallet{
		AddressChange: make(map[string]struct{}),
		AddressPublic: make(map[string]struct{}),
	}

	err := s.DB.View(func(txn *badger.Txn) error {
		err := db.Get(txn, []byte(priv), &wallet, db.HeaderWallet)
		return err
	})
	return &wallet, err
}

//NewFromPriv creates a new wallet from private key.
func NewFromPriv(s *aklib.DBConfig, pwd []byte, priv string) (*Wallet, error) {
	seed, isNode, err := address.HDFrom58(s.Config, priv, pwd)
	if err != nil {
		return nil, err
	}
	if isNode {
		return nil, errors.New("this private key is not for wallet")
	}
	encseed := address.EncryptSeed(seed, pwd)
	wallet := &Wallet{
		AccountName:   priv,
		EncSeed:       encseed,
		AddressChange: make(map[string]struct{}),
		AddressPublic: make(map[string]struct{}),
	}
	return wallet, wallet.put(s)
}

//History is a tx history for an account.
type History struct {
	Received time.Time
	*tx.InoutHash
}

//PutHistory saves histroies.
func PutHistory(s *aklib.DBConfig, hist []*History) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, nil, hist, db.HeaderWalletHistory)
	})
}

//GetHistory gets histories of addresses  in wallet.
func GetHistory(s *aklib.DBConfig) ([]*History, error) {
	var hist []*History
	return hist, s.DB.View(func(txn *badger.Txn) error {
		err := db.Get(txn, nil, &hist, db.HeaderWalletHistory)
		if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	})
}

//GetAllPrivateKeys returns privatekeys stored in the wallet.
func GetAllPrivateKeys(s *aklib.DBConfig) ([]string, error) {
	var pk []string
	err := s.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek([]byte{byte(db.HeaderWallet)}); it.ValidForPrefix([]byte{byte(db.HeaderWallet)}); it.Next() {
			pk = append(pk, string(it.Item().KeyCopy(nil)))
		}
		return nil
	})
	return pk, err
}

//Put save the wallet.
func (w *Wallet) put(s *aklib.DBConfig) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, []byte(w.AccountName), w, db.HeaderWallet)
	})
}

//DecryptSeed decrypts the seed of wallet.
func (w *Wallet) DecryptSeed(pwd []byte) ([]byte, error) {
	return address.DecryptSeed(w.EncSeed, pwd)
}

//PutAddress saves adr.
func (w *Wallet) PutAddress(s *aklib.DBConfig, pwd []byte, adr *Address, doEnc bool) error {
	if doEnc {
		adr.EncAddress = address.EncryptSeed(arypack.Marshal(adr.Address), pwd)
	} else {
		dat, err2 := address.DecryptSeed(adr.EncAddress, pwd)
		if err2 != nil {
			return err2
		}
		if err := arypack.Unmarshal(dat, &adr.Address); err != nil {
			return err
		}
	}
	name := adr.Address.Address58(s.Config)
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, []byte(name), adr, db.HeaderWalletAddress)
	})
}

//GetAddress returns an address struct.
func (w *Wallet) GetAddress(s *aklib.DBConfig, name string, pwd []byte) (*Address, error) {
	var adr Address
	return &adr, s.DB.View(func(txn *badger.Txn) error {
		if err := db.Get(txn, []byte(name), &adr, db.HeaderWalletAddress); err != nil {
			return err
		}
		dat, err2 := address.DecryptSeed(adr.EncAddress, pwd)
		if err2 != nil {
			return err2
		}
		return arypack.Unmarshal(dat, &adr.Address)
	})
}

//InitSeed initialize the seed.
func (w *Wallet) InitSeed(s *aklib.DBConfig, pwd []byte) error {
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		panic(err)
	}
	w.EncSeed = address.EncryptSeed(seed, pwd)
	return w.put(s)
}

//GetAllAddress returns all used address in DB.
func GetAllAddress(s *aklib.DBConfig) (map[string]*Address, error) {
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

//GetAllUTXO returns all UTXOs with balance.
func (w *Wallet) GetAllUTXO(s *aklib.DBConfig, pwd []byte) ([]*tx.UTXO, uint64, error) {
	log.Println(4)
	u, bal, err := w.GetUTXO(s, pwd, true)
	if err != nil {
		return nil, 0, err
	}
	log.Println(5)
	u2, bal2, err := w.GetUTXO(s, pwd, false)
	if err != nil {
		return nil, 0, err
	}
	return append(u, u2...), bal + bal2, nil
}

//GetUTXO returns UTXOs with balance.
func (w *Wallet) GetUTXO(s *aklib.DBConfig, pwd []byte, isPublic bool) ([]*tx.UTXO, uint64, error) {
	var bal uint64
	var utxos []*tx.UTXO
	adrmap := w.AddressChange
	if isPublic {
		adrmap = w.AddressPublic
	}
	for adrname := range adrmap {
		log.Println(adrname)
		adr := &Address{
			Adrstr: adrname,
		}
		if pwd != nil {
			var err error
			adr, err = w.GetAddress(s, adrname, pwd)
			if err != nil {
				return nil, 0, err
			}
		}
		hs, err := imesh.GetHisoty2(s, adrname, true)
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
				outstat := tr.OutputStatus[0][h.Index]
				if !tr.IsAccepted() ||
					(outstat.IsReferred || outstat.IsSpent || outstat.UsedByMinable != nil) {
					log.Println(h.Hash, outstat.IsReferred, outstat.IsSpent, outstat.UsedByMinable)
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
	return tr.Sign(adr.Address)
}

//Encrypt encrypts the adr.
func (adr *Address) Encrypt(pwd []byte) {
	adr.EncAddress = address.EncryptSeed(arypack.Marshal(adr.Address), pwd)
}

//NewAddress creates an address in wallet.
func (w *Wallet) NewAddress(s *aklib.DBConfig, pwd []byte, isPublic bool) (*address.Address, error) {
	adrmap := w.AddressPublic
	var idx uint32
	if !isPublic {
		adrmap = w.AddressChange
		idx = 1
	}
	master, err := address.DecryptSeed(w.EncSeed, pwd)
	if err != nil {
		return nil, err
	}
	seed := address.HDseed(master, idx, uint32(len(adrmap)))
	a, err := address.New(s.Config, seed)
	if err != nil {
		return nil, err
	}
	adr := &Address{
		Address: a,
		Adrstr:  a.Address58(s.Config),
	}
	if err := w.PutAddress(s, pwd, adr, true); err != nil {
		return nil, err
	}
	adrmap[a.Address58(s.Config)] = struct{}{}
	return a, w.put(s)
}

//NewPublicAddressFromPool get a public address from pool.
func (w *Wallet) NewPublicAddressFromPool(s *aklib.DBConfig) (string, error) {
	if len(w.Pool.Address) == 0 {
		return "", errors.New("pool is empty")
	}
	a := w.Pool.Address[0]
	w.Pool.Address = w.Pool.Address[1:]
	w.AddressPublic[a] = struct{}{}
	return a, w.put(s)
}

//FindAddress returns true if wallet has adrstr.
func (w *Wallet) FindAddress(adrstr ...string) bool {
	for _, adr := range adrstr {
		_, pub := w.AddressPublic[adr]
		_, priv := w.AddressChange[adr]
		if pub || priv {
			return true
		}
	}
	return false
}

//FindAddressByte returns true if wallet has adrstr.
func (w *Wallet) FindAddressByte(cfg *aklib.DBConfig, adrstr ...address.Bytes) (bool, error) {
	for _, adr := range adrstr {
		a, err := address.Address58(cfg.Config, adr)
		if err != nil {
			return false, err
		}
		if w.FindAddress(a) {
			return true, nil
		}
	}
	return false, nil
}

//HasAddress returns true if wallet has an address in the multisg address.
func (w *Wallet) HasAddress(s *aklib.DBConfig, out *tx.MultiSigOut) (bool, error) {
	for _, mout := range out.Addresses {
		for a := range w.AddressPublic {
			adrstr, err := address.Address58(s.Config, mout)
			if err != nil {
				return false, err
			}
			if a == adrstr {
				return true, nil
			}
		}
	}
	return false, nil
}
