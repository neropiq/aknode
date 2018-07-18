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
	"encoding/binary"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/rand"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/dgraph-io/badger"
)

var txno = struct {
	TxNo uint64
	sync.RWMutex
}{}

//TxStatus is a stutus for eeach tx(confirmed or not)
type TxStatus byte

//Status is a status for tx.
const (
	StatusPending   TxStatus = 0x00
	StatusConfirmed          = 0x01
	StatusRejected           = 0xff
)

//OutputStatus is status of an output.
type OutputStatus struct {
	IsReferred    bool   //referred from tx(s) input
	IsSpent       bool   //refered from a confirmed tx input
	UsedByMinable []byte //append(haash,type)
}

//TxInfo is for tx in db with sighash and status.
type TxInfo struct {
	Hash         tx.Hash `msgpack:"-"`
	Body         *tx.Body
	TxNo         uint64
	Status       TxStatus
	Received     time.Time
	OutputStatus [3][]OutputStatus
}

func (ti *TxInfo) sigKey() []byte {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, ti.TxNo)
	return key[:5]
}

func (ti *TxInfo) nextTxNo(s *setting.Setting, txn *badger.Txn) error {
	if err := updateTxNo(s, txn); err != nil {
		return err
	}
	ti.TxNo = txno.TxNo
	return nil
}

//PreviousOutput returns an output of the input tx.
func PreviousOutput(s *setting.Setting, in *tx.Input) (*tx.Output, error) {
	prev, err := GetTxInfo(s, in.PreviousTX)
	if err != nil {
		return nil, err
	}
	return prev.Body.Outputs[in.Index], nil
}

// Has returns true if hash exists in db.
func Has(s *setting.Setting, hash []byte) (bool, error) {
	err := s.DB.View(func(txn *badger.Txn) error {
		_, err2 := txn.Get(append([]byte{byte(db.HeaderTxInfo)}, hash...))
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

//Put puts a transaction info.
//called only from Init.
func (ti *TxInfo) put(s *setting.Setting, h tx.Hash) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Put(txn, h, ti, db.HeaderTxInfo)
	})

}

//GetTxInfo gets a transaction info.
func GetTxInfo(s *setting.Setting, h tx.Hash) (*TxInfo, error) {
	var ti TxInfo
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, h, &ti, db.HeaderTxInfo)
	})
	ti.Hash = h
	return &ti, err
}

//GetTxFunc is a func for getting tx from hash.
//This is for funcs in tx  package.
func getTxFunc(s *setting.Setting) func(hash []byte) (*tx.Body, error) {
	return func(hash []byte) (*tx.Body, error) {
		ti, err := GetTxInfo(s, hash)
		if err != nil {
			return nil, err
		}
		return ti.Body, nil
	}
}

//IsValid checks transaction tr is valid for a mamber of iMesh.
func IsValid(s *setting.Setting, tr *tx.Transaction, typ tx.Type) error {
	return tr.CheckAll(getTxFunc(s), s.Config, typ)
}

//GetTx returns a transaction  from  hash.
func GetTx(s *setting.Setting, hash []byte) (*tx.Transaction, error) {
	var sig tx.Signatures
	var ti TxInfo
	err := s.DB.View(func(txn *badger.Txn) error {
		if err2 := db.Get(txn, hash, &ti, db.HeaderTxInfo); err2 != nil {
			return err2
		}
		return db.Get(txn, ti.sigKey(), &sig, db.HeaderTxSig)
	})
	if err != nil {
		return nil, err
	}
	return &tx.Transaction{
		Body:       ti.Body,
		Signatures: sig,
	}, nil
}

//called synchonously from resolve
func putTxSub(s *setting.Setting, tr *tx.Transaction) error {
	ti := TxInfo{
		Body:     tr.Body,
		Received: time.Now(),
	}
	ti.OutputStatus[TypeIn] = make([]OutputStatus, len(tr.Outputs))
	ti.OutputStatus[TypeMulin] = make([]OutputStatus, len(tr.MultiSigOuts))
	if tr.TicketOutput != nil {
		ti.OutputStatus[TypeTicketin] = make([]OutputStatus, 1)
	}

	return s.DB.Update(func(txn *badger.Txn) error {
		if err := ti.nextTxNo(s, txn); err != nil {
			return err
		}
		if err2 := db.Put(txn, tr.Hash(), &ti, db.HeaderTxInfo); err2 != nil {
			return err2
		}
		for _, prev := range inputHashes(tr.Body) {
			var ti2 TxInfo
			if err := db.Get(txn, prev.Hash, &ti2, db.HeaderTxInfo); err != nil {
				return err
			}
			if p := &(ti2.OutputStatus[prev.Type][prev.Index].IsReferred); !*p {
				(*p) = true
				if err := db.Put(txn, prev.Hash, &ti2, db.HeaderTxInfo); err != nil {
					return err
				}
			}
			if m := ti2.OutputStatus[prev.Type][prev.Index].UsedByMinable; m != nil {
				if err := deleteMinableTx(txn, m[:32], db.Header(m[32])); err != nil {
					return err
				}
			}
		}
		if err := putAddressToTx(s, txn, tr); err != nil {
			return err
		}
		return db.Put(txn, ti.sigKey(), tr.Signatures, db.HeaderTxSig)
	})
}

//PutTx puts a transaction  into db.
func putTx(s *setting.Setting, tr *tx.Transaction) error {
	if err := tr.Check(s.Config, tx.TypeNormal); err != nil {
		return err
	}
	return putTxSub(s, tr)
}

//locked by mutex(unresolved)
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

//locked by mutex(unresolved)
func deleteUnresolvedTx(s *setting.Setting, hash []byte) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return db.Del(txn, hash, db.HeaderUnresolvedTx)
	})
}

func deleteMinableTx(txn *badger.Txn, h tx.Hash, header db.Header) error {
	var minTx tx.Transaction
	if err := db.Get(txn, h, &minTx, header); err != nil {
		return err
	}
	if err := db.Del(txn, h, header); err != nil {
		return err
	}
	for _, prev := range inputHashes(minTx.Body) {
		var ti TxInfo
		if err := db.Get(txn, prev.Hash, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
		ti.OutputStatus[prev.Type][prev.Index].UsedByMinable = nil
		if err := db.Put(txn, prev.Hash, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	return nil
}

//PutMinableTx puts a minable transaction into db.
//called synchonously from resolve.
func putMinableTx(s *setting.Setting, tr *tx.Transaction, typ tx.Type) error {
	if typ != tx.TypeRewardFee && typ != tx.TypeRewardTicket {
		return errors.New("invalid type")
	}
	if err := tr.Check(s.Config, typ); err != nil {
		return err
	}
	header, err := msg.TxType2DBHeader(typ)
	if err != nil {
		log.Fatal(err)
	}
	return s.DB.Update(func(txn *badger.Txn) error {
		var ti TxInfo
		for _, h := range inputHashes(tr.Body) {
			if err := db.Get(txn, h.Hash, &ti, db.HeaderTxInfo); err != nil {
				return err
			}
			if m := ti.OutputStatus[h.Type][h.Index].UsedByMinable; m != nil {
				if err := deleteMinableTx(txn, m[:32], db.Header(m[32])); err != nil {
					return err
				}
			}
			header2, err := msg.TxType2DBHeader(typ)
			if err != nil {
				return err
			}
			ti.OutputStatus[h.Type][h.Index].UsedByMinable = append(tr.Hash(), byte(header2))
			if err := db.Put(txn, h.Hash, &ti, db.HeaderTxInfo); err != nil {
				return err
			}
		}
		return db.Put(txn, tr.Hash(), tr, header)
	})
}

//IsMinableTxValid returns true if all inputs are not used in imesh.
func IsMinableTxValid(s *setting.Setting, tr *tx.Transaction) (bool, error) {
	for _, prev := range inputHashes(tr.Body) {
		ti, err := GetTxInfo(s, prev.Hash)
		if err != nil {
			return false, err
		}
		if ti.OutputStatus[prev.Type][prev.Index].IsReferred {
			return false, nil
		}
	}
	return true, nil
}

//GetMinableTx gets a minable transaction into db.
func GetMinableTx(s *setting.Setting, h tx.Hash, typ tx.Type) (*tx.Transaction, error) {
	if typ != tx.TypeRewardFee && typ != tx.TypeRewardTicket {
		return nil, errors.New("invalid type")
	}
	header, err := msg.TxType2DBHeader(typ)
	if err != nil {
		log.Fatal(err)
	}
	var tr tx.Transaction
	err = s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, h, &tr, header)
	})
	if err != nil {
		return nil, err
	}
	valid, err := IsMinableTxValid(s, &tr)
	if err != nil {
		return nil, err
	}
	if valid {
		return &tr, nil
	}
	return nil, errors.New("the tx is already mined")
}

func getRandomMinableTx(s *setting.Setting, header db.Header, f func(*badger.Txn, tx.Hash) (bool, error)) (*tx.Transaction, error) {
	var tr *tx.Transaction
	err := s.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		var hashes [][]byte
		for it.Seek([]byte{byte(header)}); it.ValidForPrefix([]byte{byte(header)}); it.Next() {
			h := it.Item().Key()[1:]
			ok, err2 := f(txn, h)
			if err2 != nil {
				return err2
			}
			if ok {
				hashes = append(hashes, h)
			}
		}
		if len(hashes) == 0 {
			return nil
		}
		var trr tx.Transaction
		j := rand.R.Intn(len(hashes))
		if err2 := db.Get(txn, hashes[j], &trr, header); err2 != nil {
			return err2
		}
		tr = &trr
		return nil
	})
	if tr == nil {
		return nil, errors.New("there is no minable tx")
	}
	return tr, err
}

//GetRandomFeeTx gets a fee minable transaction from db.
func GetRandomFeeTx(s *setting.Setting, min uint64) (*tx.Transaction, error) {
	return getRandomMinableTx(s, db.HeaderTxRewardFee,
		func(txn *badger.Txn, h tx.Hash) (bool, error) {
			var trr tx.Transaction
			if err2 := db.Get(txn, h, &trr, db.HeaderTxRewardFee); err2 != nil {
				return false, err2
			}
			if trr.Outputs[len(trr.Outputs)-1].Value >= min {
				return true, nil
			}
			return false, nil
		})
}

//GetRandomTicketTx gets a ticket minable transaction from db.
func GetRandomTicketTx(s *setting.Setting) (*tx.Transaction, error) {
	return getRandomMinableTx(s, db.HeaderTxRewardTicket,
		func(txn *badger.Txn, h tx.Hash) (bool, error) {
			return true, nil
		})
}

//locked by mutex(unresolved)
func putBrokenTx(s *setting.Setting, h tx.Hash) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		err := txn.SetWithTTL(append([]byte{byte(db.HeaderBrokenTx)}, h...), nil, 24*time.Hour)
		if err == badger.ErrConflict {
			return nil
		}
		return err
	})
}

func isBrokenTx(s *setting.Setting, h []byte) (bool, error) {
	err := s.DB.View(func(txn *badger.Txn) error {
		_, err := txn.Get(append([]byte{byte(db.HeaderBrokenTx)}, h...))
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

//locked by mutex
func updateTxNo(s *setting.Setting, txn *badger.Txn) error {
	txno.Lock()
	defer txno.Unlock()
	txno.TxNo++
	return db.Put(txn, nil, &txno.TxNo, db.HeaderTxNo)
}

func getTxNo(s *setting.Setting) error {
	return s.DB.View(func(txn *badger.Txn) error {
		err := db.Get(txn, nil, &txno.TxNo, db.HeaderTxNo)
		if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	})
}

//GetTxNo returns a total number of txs in imesh.
func GetTxNo() uint64 {
	txno.RLock()
	defer txno.RUnlock()
	return txno.TxNo
}
