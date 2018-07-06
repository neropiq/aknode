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

//Types for in/out txs.
const (
	TypeIn = iota
	TypeMulin
	TypeTicketin
	TypeOut
	TypeMulout
	TypeTicketout
)

//InoutHash represents in/out tx hashess.
type InoutHash struct {
	Hash  tx.Hash
	Type  byte
	Index byte
}

func newInoutHash(dat []byte) (*InoutHash, error) {
	if len(dat) != 34 {
		return nil, errors.New("invalid dat length")
	}
	ih := &InoutHash{
		Hash:  make(tx.Hash, 32),
		Type:  dat[32],
		Index: dat[33],
	}
	copy(ih.Hash, dat[:32])
	return ih, nil
}
func (ih *InoutHash) bytes() []byte {
	return inout2key(ih.Hash, ih.Type, ih.Index)
}
func inout2key(h tx.Hash, typ, no byte) []byte {
	var r [34]byte
	copy(r[:], h)
	r[32] = typ
	r[33] = no
	return r[:]
}

func inputHashes(tr *tx.Body) []*InoutHash {
	prevs := make([]*InoutHash, 0, 1+
		len(tr.Inputs)+len(tr.MultiSigIns))
	if tr.TicketInput != nil {
		prevs = append(prevs, &InoutHash{
			Type: TypeTicketin,
			Hash: tr.TicketInput,
		})
	}
	for _, prev := range tr.Inputs {
		prevs = append(prevs, &InoutHash{
			Type: TypeIn,
			Hash: prev.PreviousTX,
		})
	}
	for _, prev := range tr.MultiSigIns {
		prevs = append(prevs, &InoutHash{
			Type: TypeMulin,
			Hash: prev.PreviousTX,
		})
	}
	return prevs
}

//OutputStatus is status of an output.
type OutputStatus struct {
	IsReferred    bool //referred from tx(s) input
	IsSpent       bool //refered from a confirmed tx input
	UsedByMinable map[[32]byte]db.Header
}

//TxInfo is for tx in db with sighash and status.
type TxInfo struct {
	Body         *tx.Body
	SigNo        uint64
	Status       byte
	OutputStatus [3][]OutputStatus
}

func (ti *TxInfo) sigKey() []byte {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, ti.SigNo)
	return key[:5]
}

func (ti *TxInfo) nextSigKey(s *setting.Setting) error {
	seq, err := s.DB.GetSequence([]byte("sequence"), (1<<40)-1)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := seq.Release(); err2 != nil {
			log.Fatal(err2)
		}
	}()
	no, err := seq.Next()
	if err != nil {
		return err
	}
	ti.SigNo = no
	return nil
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
	return &ti, err
}

//GetTxFunc is a func for getting tx from hash.
//This is for funcs in tx  package.
func GetTxFunc(s *setting.Setting) func(hash []byte) (*tx.Body, error) {
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
	return tr.CheckAll(GetTxFunc(s), s.Config, typ)
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
		Body: tr.Body,
	}
	if err := ti.nextSigKey(s); err != nil {
		return err
	}
	ti.OutputStatus[TypeIn] = make([]OutputStatus, len(tr.Outputs))
	ti.OutputStatus[TypeMulin] = make([]OutputStatus, len(tr.MultiSigOuts))
	if tr.TicketOutput != nil {
		ti.OutputStatus[TypeTicketin] = make([]OutputStatus, 1)
	}

	return s.DB.Update(func(txn *badger.Txn) error {
		if err2 := db.Put(txn, tr.Hash(), &ti, db.HeaderTxInfo); err2 != nil {
			return err2
		}
		for _, prev := range inputHashes(tr.Body) {
			var ti2 TxInfo
			if err := db.Get(txn, prev.Hash, &ti2, db.HeaderTxInfo); err != nil {
				return err
			}
			if !ti2.OutputStatus[prev.Type][prev.Index].IsReferred {
				ti2.OutputStatus[prev.Type][prev.Index].IsReferred = true
				if err := db.Put(txn, prev.Hash, &ti2, db.HeaderTxInfo); err != nil {
					return err
				}
			}
			for h, header := range ti2.OutputStatus[prev.Type][prev.Index].UsedByMinable {
				if err := deleteMinableTx(txn, h[:], header); err != nil {
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
	if err := tr.Check(s.Config, tx.TxNormal); err != nil {
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
		delete(ti.OutputStatus[prev.Type][prev.Index].UsedByMinable, h.Array())
		if err := db.Put(txn, prev.Hash, &ti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	return nil
}

//PutMinableTx puts a minable transaction into db.
//called synchonously from resolve.
func putMinableTx(s *setting.Setting, tr *tx.Transaction, typ tx.Type) error {
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
		for _, h := range inputHashes(tr.Body) {
			if err := db.Get(txn, h.Hash, &ti, db.HeaderTxInfo); err != nil {
				return err
			}
			if ti.OutputStatus[h.Type][h.Index].UsedByMinable == nil {
				ti.OutputStatus[h.Type][h.Index].UsedByMinable = make(map[[32]byte]db.Header)
			}
			header2, err := msg.TxType2DBHeader(typ)
			if err != nil {
				return err
			}
			ti.OutputStatus[h.Type][h.Index].UsedByMinable[tr.Hash().Array()] = header2
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
		if err2 := db.Get(txn, hashes[j][1:], &trr, header); err2 != nil {
			return err2
		}
		tr = &trr
		return nil
	})
	return tr, err
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
