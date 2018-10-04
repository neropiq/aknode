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

package akconsensus

import (
	"bytes"
	"errors"
	"sync"
	"time"

	"github.com/AidosKuneen/aknode/imesh"

	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/AidosKuneen/consensus"
	"github.com/dgraph-io/badger"
)

var (
	notify      chan []tx.Hash
	proposals   = make(map[consensus.ProposalID]time.Time)
	validations = make(map[consensus.ValidationID]time.Time)
	lastLedger  *consensus.Ledger
	mutex       sync.RWMutex
)

type network interface {
	GetLedger(s *setting.Setting, id consensus.LedgerID)
	BroadcastProposal(s *setting.Setting, p *consensus.Proposal)
	BroadcastValidatoin(s *setting.Setting, v *consensus.Validation)
}

//LastLedger returns the last ledger.
func LastLedger() *consensus.Ledger {
	mutex.RLock()
	defer mutex.RUnlock()
	return lastLedger
}

//Init initialize consensus.
func Init(s *setting.Setting) error {
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, nil, &lastLedger, db.HeaderLastLedger)
	})
	return err
}

//HandleValidation checks p was already received or not, and
//p is from a trusted node.
func handleValidation(s *setting.Setting, peer *consensus.Peer, p *consensus.Validation) (bool, error) {
	id := p.ID()
	var sig address.Signature
	if err := arypack.Unmarshal(p.Signature, &sig); err != nil {
		return false, err
	}
	if err := sig.Verify(id[:]); err != nil {
		return false, err
	}
	adr := sig.Address(s.Config, true)
	if !bytes.Equal(adr[2:], p.NodeID[:]) {
		return false, errors.New("invalid nodeID")
	}
	if _, ok := validations[p.ID()]; ok {
		return false, nil
	}
	validations[p.ID()] = time.Now()
	for k, v := range validations {
		if time.Now().After(v.Add(time.Hour)) {
			delete(validations, k)
		}
	}
	adr = append(s.Config.PrefixNode, p.NodeID[:]...)
	adrstr, err := address.Address58(s.Config, adr)
	if err != nil {
		return false, err
	}
	ok := true
	if s.IsTrusted(adrstr) {
		ok = peer.AddValidation(p)
	}
	return ok, nil
}

//HandleProposal checks p was already received or not, and
//p is from a trusted node.
func handleProposal(s *setting.Setting, peer *consensus.Peer, p *consensus.Proposal) (bool, error) {
	id := p.ID()
	var sig address.Signature
	if err := arypack.Unmarshal(p.Signature, &sig); err != nil {
		return false, err
	}
	if err := sig.Verify(id[:]); err != nil {
		return false, err
	}
	adr := sig.Address(s.Config, true)
	if !bytes.Equal(adr[2:], p.NodeID[:]) {
		return false, errors.New("invalid nodeID")
	}

	if _, ok := proposals[p.ID()]; ok {
		return false, nil
	}
	proposals[p.ID()] = time.Now()
	for k, v := range proposals {
		if time.Now().After(v.Add(time.Hour)) {
			delete(proposals, k)
		}
	}
	adr = append(s.Config.PrefixNode, p.NodeID[:]...)
	adrstr, err := address.Address58(s.Config, adr)
	if err != nil {
		return false, err
	}
	if s.IsTrusted(adrstr) {
		peer.AddProposal(p)
	}
	return true, nil
}

//PutLedger puts a ledger.
func PutLedger(s *setting.Setting, l *consensus.Ledger) error {
	mutex.Lock()
	defer mutex.Unlock()
	return s.DB.Update(func(txn *badger.Txn) error {
		id := l.ID()
		if err := db.Put(txn, id[:], l, db.HeaderLedger); err != nil {
			return err
		}
		return db.Put(txn, nil, lastLedger, db.HeaderLastLedger)
	})
}

//GetLedger gets a ledger whose ID is id.
func GetLedger(s *setting.Setting, id consensus.LedgerID) (*consensus.Ledger, error) {
	mutex.RLock()
	defer mutex.RUnlock()
	var l consensus.Ledger
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, id[:], &l, db.HeaderLedger)
	})
	return &l, err
}

//ReadGetLeadger parse a getLedger command.
func ReadGetLeadger(buf []byte) (consensus.LedgerID, error) {
	var v consensus.LedgerID
	err := arypack.Unmarshal(buf, &v)
	return v, err
}

//ReadLeadger parse a Ledger command.
func ReadLeadger(peer *consensus.Peer, buf []byte) (*consensus.Ledger, error) {
	var v consensus.Ledger
	err := arypack.Unmarshal(buf, &v)
	v.IndexOf = peer.IndexOfFunc(&v)
	return &v, err
}

//ReadValidation parse a Validation command.
func ReadValidation(s *setting.Setting, peer *consensus.Peer, buf []byte) (*consensus.Validation, bool, error) {
	var v consensus.Validation
	err := arypack.Unmarshal(buf, &v)
	if err != nil {
		return nil, false, err
	}
	mutex.Lock()
	noexist, err := handleValidation(s, peer, &v)
	mutex.Unlock()
	return &v, noexist, err
}

//ReadProposal parse a Proposal command.
func ReadProposal(s *setting.Setting, peer *consensus.Peer, buf []byte) (*consensus.Proposal, bool, error) {
	var v consensus.Proposal
	err := arypack.Unmarshal(buf, &v)
	if err != nil {
		return nil, false, err
	}
	mutex.Lock()
	noexist, err := handleProposal(s, peer, &v)
	mutex.Unlock()
	return &v, noexist, err
}

//Confirm confirms txs and return hashes of confirmed txs.
func Confirm(s *setting.Setting, peer network, l *consensus.Ledger) error {
	mutex.Lock()
	defer mutex.Unlock()
	var ctx tx.Hash
	for h := range l.Txs {
		ctx = tx.Hash(h[:])
	}
	var tr []tx.Hash

	seq := consensus.NewSpan(l).Diff(lastLedger)
	last := lastLedger
	//get all ledgers
	for i := l.Seq; i != seq; i-- {
		if _, err := GetLedger(s, last.ParentID); err == badger.ErrKeyNotFound {
			peer.GetLedger(s, last.ParentID)
			time.Sleep(10 * time.Second)
		}
		var err error
		last, err = GetLedger(s, last.ParentID)
		if err != nil {
			return err
		}
	}
	//go backward
	last = lastLedger
	for i := l.Seq; i != seq; i-- {
		var t tx.Hash
		for h := range last.Txs {
			t = tx.Hash(h[:])
		}
		if err := imesh.RevertConfirmation(s, t, imesh.StatNo(last.ID())); err != nil {
			return err
		}
		lastLedger = last
		var err error
		last, err = GetLedger(s, last.ParentID)
		if err != nil {
			return err
		}
	}
	//go forward
	for i := seq; i <= l.Seq; i++ {
		id := l.IndexOf(i)
		ll, err := GetLedger(s, id)
		if err != nil {
			return err
		}
		var t tx.Hash
		for h := range ll.Txs {
			t = tx.Hash(h[:])
		}
		if err := imesh.Confirm(s, t, l.ID()); err != nil {
			return err
		}
	}

	if notify != nil {
		txs := make([]tx.Hash, len(tr))
		copy(txs, tr)
		notify <- txs
	}
	if err := leaves.SetConfirmed(s, ctx); err != nil {
		return err
	}
	lastLedger = l
	return PutLedger(s, l)
}

//RegisterTxNotifier registers a notifier for resolved txs.
func RegisterTxNotifier(n chan []tx.Hash) {
	notify = n
}
