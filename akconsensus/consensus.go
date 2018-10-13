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
	"log"
	"sync"
	"time"

	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/AidosKuneen/consensus"
	"github.com/dgraph-io/badger"
)

var (
	notify            chan []tx.Hash
	proposals         = make(map[consensus.ProposalID]time.Time)
	validations       = make(map[consensus.ValidationID]time.Time)
	latestLedger      = consensus.Genesis
	latestSolidLedger = consensus.Genesis
	peer              network
	mutex             sync.RWMutex
)

type ledger struct {
	ParentID            consensus.LedgerID
	Seq                 consensus.Seq
	Txs                 tx.Hash
	CloseTimeResolution time.Duration
	CloseTime           time.Time
	ParentCloseTime     time.Time
	CloseTimeAgree      bool
}

func newLedger(l *consensus.Ledger) *ledger {
	le := &ledger{
		ParentID:            l.ParentID,
		Seq:                 l.Seq,
		CloseTimeResolution: l.CloseTimeResolution,
		CloseTime:           l.CloseTime,
		ParentCloseTime:     l.ParentCloseTime,
		CloseTimeAgree:      l.CloseTimeAgree,
	}
	for h := range l.Txs {
		le.Txs = tx.Hash(h[:])
	}
	return le
}
func fromLedger(s *setting.Setting, l *ledger) (*consensus.Ledger, error) {
	var id consensus.TxID
	copy(id[:], l.Txs)
	le := &consensus.Ledger{
		ParentID:            l.ParentID,
		Seq:                 l.Seq,
		CloseTimeResolution: l.CloseTimeResolution,
		CloseTime:           l.CloseTime,
		ParentCloseTime:     l.ParentCloseTime,
		CloseTimeAgree:      l.CloseTimeAgree,
	}
	le.IndexOf = consensus.IndexOfFunc(le, func(lid consensus.LedgerID) (*consensus.Ledger, error) {
		return getLedger(s, lid)
	})
	if l.Txs != nil {
		t, err := imesh.GetTx(s.DB, l.Txs)
		if err != nil {
			return nil, err
		}
		le.Txs = consensus.TxSet{
			id: t,
		}
	}

	return le, nil
}

type network interface {
	GetLedger(s *setting.Setting, id consensus.LedgerID)
	BroadcastProposal(s *setting.Setting, p *consensus.Proposal)
	BroadcastValidatoin(s *setting.Setting, v *consensus.Validation)
}

//LatestLedger returns the last ledger.
func LatestLedger() *consensus.Ledger {
	mutex.RLock()
	defer mutex.RUnlock()
	return latestSolidLedger
}

//Init initialize consensus.
func Init(s *setting.Setting, p network) error {
	peer = p
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, nil, &latestSolidLedger, db.HeaderLastLedger)
	})
	latestLedger = latestSolidLedger
	if err == badger.ErrKeyNotFound {
		return nil
	}
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
	return putLedger(s, l)
}

func putLedger(s *setting.Setting, l *consensus.Ledger) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		id := l.ID()
		if err := db.Put(txn, id[:], newLedger(l), db.HeaderLedger); err != nil {
			return err
		}
		return db.Put(txn, nil, latestSolidLedger, db.HeaderLastLedger)
	})
}

//GetLedger gets a ledger whose ID is id.
func GetLedger(s *setting.Setting, id consensus.LedgerID) (*consensus.Ledger, error) {
	mutex.RLock()
	defer mutex.RUnlock()
	return getLedger(s, id)
}

func getLedger(s *setting.Setting, id consensus.LedgerID) (*consensus.Ledger, error) {
	if id == consensus.GenesisID {
		return consensus.Genesis, nil
	}
	var l ledger
	err := s.DB.View(func(txn *badger.Txn) error {
		return db.Get(txn, id[:], &l, db.HeaderLedger)
	})
	if err != nil {
		return nil, err
	}
	return fromLedger(s, &l)
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
	v.IndexOf = consensus.IndexOfFunc(&v, peer.AcquireLedger)
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

func goRetryLedger(s *setting.Setting) {
	go func() {
		for {
			time.Sleep(5 * time.Second)
			if latestLedger.ID() == latestSolidLedger.ID() {
				continue
			}
			if err := Confirm(s, latestLedger); err != nil {
				log.Println(err)
			}
		}
	}()
}

//Confirm confirms txs and return hashes of confirmed txs.
func Confirm(s *setting.Setting, l *consensus.Ledger) error {
	mutex.Lock()
	defer mutex.Unlock()
	latestLedger = l
	var ctx tx.Hash
	for h := range l.Txs {
		ctx = tx.Hash(h[:])
	}
	var tr []tx.Hash

	if err := putLedger(s, l); err != nil {
		return err
	}

	seq := consensus.NewSpan(l).Diff(latestSolidLedger)
	last := latestSolidLedger
	//get all ledgers
	for i := l.Seq; i > seq; i-- {
		if _, err := getLedger(s, last.ParentID); err == badger.ErrKeyNotFound {
			peer.GetLedger(s, last.ParentID)
			time.Sleep(10 * time.Second)
		}
		var err error
		last, err = getLedger(s, last.ParentID)
		if err != nil {
			return err
		}
	}
	//go backward
	last = latestSolidLedger
	for i := last.Seq; i >= seq; i-- {
		var t tx.Hash
		for h := range last.Txs {
			t = tx.Hash(h[:])
		}
		_, err := imesh.RevertConfirmation(s, t, imesh.StatNo(last.ID()))
		if err != nil {
			return err
		}
		latestSolidLedger = last
		last, err = getLedger(s, last.ParentID)
		if err != nil {
			return err
		}
	}
	//go forward
	for i := seq; i <= l.Seq; i++ {
		id := l.IndexOf(i)
		ll, err := getLedger(s, id)
		if err != nil {
			return err
		}
		var t tx.Hash
		for h := range ll.Txs {
			t = tx.Hash(h[:])
		}
		has, err := imesh.Has(s, t)
		if err != nil {
			return err
		}
		if !has {
			if err := imesh.AddNoexistTxHash(s, t, tx.TypeNormal); err != nil {
				return err
			}
			return errors.New("no tx:" + t.String())
		}
		hs, err := imesh.Confirm(s, t, l.ID())
		if err != nil {
			return err
		}
		tr = append(tr, hs...)
		latestSolidLedger = ll
	}

	if notify != nil {
		txs := make([]tx.Hash, 0, len(tr))
		for _, t := range tr {
			ti, err := imesh.GetTxInfo(s.DB, t)
			if err != nil {
				return err
			}
			if ti.IsAccepted() {
				txs = append(txs, t)
			}
		}
		notify <- txs
	}
	return leaves.SetConfirmed(s, ctx)
}

//RegisterTxNotifier registers a notifier for resolved txs.
func RegisterTxNotifier(n chan []tx.Hash) {
	mutex.Lock()
	defer mutex.Unlock()
	notify = n
}

//SetLatest is only for test. Don't use it.
func SetLatest(l *consensus.Ledger) {
	latestLedger = l
	latestSolidLedger = l
}
