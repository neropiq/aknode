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
	"errors"
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
	StatusPending     = 0x00
	StatusConfirmed   = 0x01
	StatusDoubleSpend = 0xf0
	StatusRejected    = 0xff
)

const (
	inoutTypeInputOutput byte = iota
	inoutTypeMultiSig
	inoutTypeTicket
)

//OutputStatus is status of an output.
type OutputStatus struct {
	IsUsed        bool
	ConfirmedBy   tx.Hash
	UsedByMinable map[[32]byte]db.Header
}

//TxInfo is for tx in db with sighash and status.
type TxInfo struct {
	BodyHash     tx.Hash
	SigHash      tx.Hash
	Status       byte
	OutputStatus [3][]OutputStatus
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
func (ti *TxInfo) Put(s *setting.Setting, h tx.Hash) error {
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
	return &ti, err
}

//GetTxBody returns a transaction Body from  hash.
func GetTxBody(s *setting.Setting, hash []byte) (*tx.Body, error) {
	var body tx.Body
	err := s.DB.View(func(txn *badger.Txn) error {
		var ti TxInfo
		if err2 := db.Get(txn, hash, &ti, db.HeaderTxInfo); err2 != nil {
			return err2
		}
		return db.Get(txn, ti.BodyHash, &body, db.HeaderTxBody)
	})
	return &body, err
}

func getTxFunc(s *setting.Setting) func(hash []byte) (*tx.Body, error) {
	return func(hash []byte) (*tx.Body, error) {
		body, err := GetTxBody(s, hash)
		if err != nil {
			return nil, err
		}
		return body, nil
	}
}

//IsValid checks transaction tr is valid for a mamber of iMesh.
func IsValid(s *setting.Setting, tr *tx.Transaction, typ tx.Type) error {
	return tr.CheckAll(getTxFunc(s), s.Config, typ)
}

//GetTx returns a transaction  from  hash.
func GetTx(s *setting.Setting, hash []byte) (*tx.Transaction, error) {
	var body tx.Body
	var sig tx.Signatures
	var ti TxInfo
	err := s.DB.View(func(txn *badger.Txn) error {
		if err2 := db.Get(txn, hash, &ti, db.HeaderTxInfo); err2 != nil {
			return err2
		}
		if err2 := db.Get(txn, ti.BodyHash, &body, db.HeaderTxBody); err2 != nil {
			return err2
		}
		return db.Get(txn, ti.SigHash, &sig, db.HeaderTxSig)
	})
	if err != nil {
		return nil, err
	}
	return &tx.Transaction{
		Body:       &body,
		Signatures: sig,
	}, nil
}

//PutTx puts a transaction  into db.
func putTx(s *setting.Setting, tr *tx.Transaction) error {
	ti := TxInfo{
		BodyHash: tr.Body.Hash(),
		SigHash:  tr.Signatures.Hash(),
	}
	ti.OutputStatus[inoutTypeInputOutput] = make([]OutputStatus, len(tr.Outputs))
	ti.OutputStatus[inoutTypeMultiSig] = make([]OutputStatus, len(tr.MultiSigOuts))
	if tr.TicketInput != nil {
		ti.OutputStatus[inoutTypeTicket] = make([]OutputStatus, 1)
	}

	return s.DB.Update(func(txn *badger.Txn) error {
		if err2 := db.Put(txn, tr.Hash(), &ti, db.HeaderTxInfo); err2 != nil {
			return err2
		}
		if err2 := db.Put(txn, ti.BodyHash, tr.Body, db.HeaderTxBody); err2 != nil {
			return err2
		}
		for _, prev := range inputHashes(tr) {
			var ti TxInfo
			if err := db.Get(txn, prev.Hash, &ti, db.HeaderTxInfo); err != nil {
				return err
			}
			if !ti.OutputStatus[prev.Type][prev.Index].IsUsed {
				ti.OutputStatus[prev.Type][prev.Index].IsUsed = true
				if err := db.Put(txn, prev.Hash, &ti, db.HeaderTxInfo); err != nil {
					return err
				}
			}
			for h, header := range ti.OutputStatus[prev.Type][prev.Index].UsedByMinable {
				if err := deleteMinableTx(txn, h[:], header); err != nil {
					return err
				}
			}
		}
		return db.Put(txn, ti.SigHash, tr.Signatures, db.HeaderTxSig)
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

func deleteMinableTx(txn *badger.Txn, h tx.Hash, header db.Header) error {
	var minTx tx.Transaction
	if err := db.Get(txn, h, &minTx, header); err != nil {
		return err
	}
	if err := db.Del(txn, h, header); err != nil {
		return err
	}
	for _, prev := range inputHashes(&minTx) {
		var ti TxInfo
		if err := db.Get(txn, prev.Hash, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
		delete(ti.OutputStatus[prev.Type][prev.Index].UsedByMinable, h.Array())
		if err := db.Put(txn, prev.Hash, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	return nil
}

//PutMinableTx puts a minable transaction into db.
func PutMinableTx(s *setting.Setting, tr *tx.Transaction, typ tx.Type) error {
	if typ != tx.TxRewardFee && typ != tx.TxRewardTicket {
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
		for _, h := range inputHashes(tr) {
			if err := db.Get(txn, h.Hash, &ti, db.HeaderTxInfo); err != nil {
				return err
			}
			if ti.OutputStatus[h.Type][h.Index].UsedByMinable == nil {
				ti.OutputStatus[h.Type][h.Index].UsedByMinable = make(map[[32]byte]db.Header)
			}
			header, err := msg.TxType2DBHeader(typ)
			if err != nil {
				return err
			}
			ti.OutputStatus[h.Type][h.Index].UsedByMinable[tr.Hash().Array()] = header
			if err := db.Put(txn, h.Hash, &ti, db.HeaderTxInfo); err != nil {
				return err
			}
		}
		return db.Put(txn, tr.Hash(), tr, header)
	})
}

//IsMinableTxValid returns true if all inputs are not used in imesh.
func IsMinableTxValid(s *setting.Setting, tr *tx.Transaction) (bool, error) {
	for _, prev := range inputHashes(tr) {
		ti, err := GetTxInfo(s, prev.Hash)
		if err != nil {
			return false, err
		}
		if ti.OutputStatus[prev.Type][prev.Index].IsUsed {
			return false, nil
		}
	}
	return true, nil
}

//GetMinableTx gets a minable transaction into db.
func GetMinableTx(s *setting.Setting, h tx.Hash, typ tx.Type) (*tx.Transaction, error) {
	if typ != tx.TxRewardFee && typ != tx.TxRewardTicket {
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
	return nil, errors.New("the tx is already invalid")
}

//GetRandomMinableTx gets a minable transaction from db.
//The return is nil if not found.
func GetRandomMinableTx(s *setting.Setting, typ tx.Type) (*tx.Transaction, error) {
	if typ != tx.TxRewardFee && typ != tx.TxRewardTicket {
		return nil, errors.New("invalid type")
	}
	header, err := msg.TxType2DBHeader(typ)
	if err != nil {
		log.Fatal(err)
	}
	var tr *tx.Transaction
	err = s.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		var hashes [][]byte
		for it.Seek([]byte{byte(header)}); it.ValidForPrefix([]byte{byte(header)}); it.Next() {
			if err != nil {
				return err
			}
			hashes = append(hashes, it.Item().Key())
		}
		if len(hashes) == 0 {
			return nil
		}
		var trr tx.Transaction
		j := rand.R.Intn(len(hashes))
		if err := db.Get(txn, hashes[j][1:], &trr, header); err != nil {
			return err
		}
		tr = &trr
		return nil
	})
	return tr, err
}

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

type inputHash struct {
	Type  byte
	Hash  tx.Hash
	Index byte
}

func inputHashes(tr *tx.Transaction) []*inputHash {
	prevs := make([]*inputHash, 0, 1+
		len(tr.Inputs)+len(tr.MultiSigIns))
	if tr.TicketInput != nil {
		prevs = append(prevs, &inputHash{
			Type: inoutTypeTicket,
			Hash: tr.TicketInput,
		})
	}
	for _, prev := range tr.Inputs {
		prevs = append(prevs, &inputHash{
			Type: inoutTypeInputOutput,
			Hash: prev.PreviousTX,
		})
	}
	for _, prev := range tr.MultiSigIns {
		prevs = append(prevs, &inputHash{
			Type: inoutTypeMultiSig,
			Hash: prev.PreviousTX,
		})
	}
	return prevs
}
