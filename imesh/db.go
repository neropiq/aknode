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

package imesh

import (
	"log"
	"time"

	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/rand"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/dgraph-io/badger"
)

//Status is a status for tx.
const (
	StatusPending = iota
	StatusConfirmed
	StatusDoubleSpend = iota + 0xf0
	StatusRejected    = 0xff
)

//Transaction is for tx in db with sighash and status.
type Transaction struct {
	*tx.Body
	SigHash []byte
	Status  byte
}

// Has returns true if hash exists in db.
func Has(s *setting.Setting, hash []byte) (bool, error) {
	err := s.DB.View(func(txn *badger.Txn) error {
		_, err2 := txn.Get(append([]byte{db.HeaderTxBody}, hash...))
		return err2
	})
	switch err {
	case nil:
		return true, nil
	case badger.ErrKeyNotFound:
		return false, nil
	default:
		return false, err
	}
}

//GetTxBody returns a transaction Body from  hash.
func GetTxBody(s *setting.Setting, hash []byte) (*Transaction, error) {
	var tx Transaction
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, hash, &tx, db.HeaderTxBody)
	})
	return &tx, err
}

func getTxFunc(s *setting.Setting) func(hash []byte) (*tx.Body, error) {
	return func(hash []byte) (*tx.Body, error) {
		t, err := GetTxBody(s, hash)
		if err != nil {
			return nil, err
		}
		return t.Body, nil
	}
}

//GetTx returns a transaction  from  hash.
func GetTx(s *setting.Setting, hash []byte) (*tx.Transaction, error) {
	var body Transaction
	var sig tx.Signatures
	err := s.DB.View(func(txn *badger.Txn) error {
		if err2 := db.Get(txn, hash, &body, db.HeaderTxBody); err2 != nil {
			return err2
		}
		return db.Get(txn, body.SigHash, &sig, db.HeaderTxSig)
	})
	if err != nil {
		return nil, err
	}
	return &tx.Transaction{
		Body:       body.Body,
		Signatures: sig,
	}, nil
}

//PutTx puts a transaction  into db.
func putTx(s *setting.Setting, tx *tx.Transaction) error {
	body := Transaction{
		Body:    tx.Body,
		SigHash: tx.Signatures.Hash(),
	}
	return s.DB.Update(func(txn *badger.Txn) error {
		if err2 := db.Put(txn, tx.Hash(), &body, db.HeaderTxBody); err2 != nil {
			return err2
		}
		if err2 := db.Get(txn, body.SigHash, &tx.Signatures, db.HeaderTxSig); err2 != nil {
			return err2
		}
		for _, k := range []byte{db.HeaderFeeTx, db.HeaderTicketTx} {
			key := append([]byte{k}, tx.PreHash()...)
			if _, err := txn.Get(key); err != badger.ErrKeyNotFound {
				if err2 := txn.Delete(key); err2 != nil {
					return err2
				}
			}
		}
		return nil
	})
}

func putUnresolvedTx(s *setting.Setting, tx *tx.Transaction) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, tx.Hash(), tx, db.HeaderUnresolvedTx)
	})
}

func getUnresolvedTx(s *setting.Setting, hash []byte) (*tx.Transaction, error) {
	var tr tx.Transaction
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, hash, &tr, db.HeaderUnresolvedTx)
	})
	return &tr, err
}

func deleteUnresolvedTx(s *setting.Setting, hash []byte) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Del(txn, hash, db.HeaderUnresolvedTx)
	})
}

//PutMinableTx puts a minable transaction into db.
func PutMinableTx(s *setting.Setting, tr *tx.Transaction) error {
	typ, err := tr.CheckMinable(s.Config)
	if err != nil {
		return err
	}
	header, err := msg.InvType(typ)
	if err != nil {
		log.Fatal(err)
	}
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, tr.PreHash(), tr, header)
	})
}

//GetRandomMinableTx gets a minable transaction from db.
func GetRandomMinableTx(s *setting.Setting, typ byte) (*tx.Transaction, error) {
	header, err := msg.InvType(typ)
	if err != nil {
		log.Fatal(err)
	}
	var tr tx.Transaction
	err = s.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		var hashes [][]byte
		for it.Seek([]byte{header}); it.ValidForPrefix([]byte{header}); it.Next() {
			hashes = append(hashes, it.Item().Key())
		}
		j := rand.R.Intn(len(hashes))
		return db.Get(txn, hashes[j], &tr, header)
	})
	return &tr, err
}

//GetMinableTx gets a minable transaction from db.
func GetMinableTx(s *setting.Setting, h tx.Hash) (*tx.Transaction, error) {
	var tr tx.Transaction
	err := s.DB.View(func(txn *badger.Txn) error {
		err0 := db.Get(txn, h, &tr, db.HeaderFeeTx)
		if err0 != nil && err0 != badger.ErrKeyNotFound {
			return err0
		}
		if err0 == nil {
			return nil
		}
		return db.Get(txn, h, &tr, db.HeaderTicketTx)
	})
	return &tr, err
}

func putBrokenTx(s *setting.Setting, h tx.Hash) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		err := txn.SetWithTTL(append([]byte{db.HeaderBrokenTx}, h...), nil, 24*time.Hour)
		if err == badger.ErrConflict {
			return nil
		}
		return err
	})
}

func isBrokenTx(s *setting.Setting, h []byte) (bool, error) {
	err := s.DB.View(func(txn *badger.Txn) error {
		_, err := txn.Get(append([]byte{db.HeaderBrokenTx}, h...))
		return err
	})
	if err == nil {
		return true, nil
	}
	if err == badger.ErrKeyNotFound {
		return false, nil
	}
	return false, err
}
