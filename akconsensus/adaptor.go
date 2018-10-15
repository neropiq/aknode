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
	"encoding/hex"
	"errors"
	"log"
	"sort"
	"time"

	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/AidosKuneen/consensus"
)

type network interface {
	GetLedger(s *setting.Setting, id consensus.LedgerID)
	BroadcastProposal(s *setting.Setting, p *consensus.Proposal)
	BroadcastValidatoin(s *setting.Setting, v *consensus.Validation)
}

//Adaptor is an adaptor for consensus.
type Adaptor struct {
	s *setting.Setting
}

//NewAdaptor returns a instance of Adaptor.
func NewAdaptor(s *setting.Setting) *Adaptor {
	return &Adaptor{
		s: s,
	}
}

// AcquireLedger attempts to acquire a specific ledger.
func (a *Adaptor) AcquireLedger(id consensus.LedgerID) (*consensus.Ledger, error) {
	l, err := GetLedger(a.s, id)
	if err == nil {
		return l, nil
	}
	peer.GetLedger(a.s, id)
	return nil, errors.New("not found")
}

//OnStale handles a newly stale validation, this should do minimal work since
//it is called by Validations while it may be iterating Validations
//under lock
func (a *Adaptor) OnStale(*consensus.Validation) {} //nothing

// Flush the remaining validations (typically done on shutdown)
func (a *Adaptor) Flush(remaining map[consensus.NodeID]*consensus.Validation) {} //nothing

// AcquireTxSet acquires the transaction set associated with a proposed position.
func (a *Adaptor) AcquireTxSet(id consensus.TxSetID) ([]consensus.TxT, error) {
	tx, err := imesh.GetTx(a.s.DB, id[:])
	if err != nil {
		return nil, err
	}
	return []consensus.TxT{tx}, nil
}

// HasOpenTransactions returns whether any transactions are in the open ledger
func (a *Adaptor) HasOpenTransactions() bool {
	ls := leaves.GetAllUnconfirmed()
	return len(ls) > 0
}

//OnModeChange is called whenever consensus operating mode changes
func (a *Adaptor) OnModeChange(consensus.Mode, consensus.Mode) {} //nothing

// OnClose is called when ledger closes
func (a *Adaptor) OnClose(prev *consensus.Ledger, now time.Time, mode consensus.Mode) consensus.TxSet {
	//return the oldest unconfirmed leaf
	ls := leaves.GetAllUnconfirmed()
	if len(ls) == 0 {
		return nil
	}
	trs := make([]*imesh.TxInfo, len(ls))
	var err error
	for i, h := range ls {
		trs[i], err = imesh.GetTxInfo(a.s.DB, h)
		if err != nil {
			log.Println(err)
			return nil
		}
	}
	sort.Slice(trs, func(i, j int) bool {
		return trs[i].Received.Before(trs[j].Received)
	})
	tr, err := imesh.GetTx(a.s.DB, trs[0].Hash)
	if err != nil {
		log.Println(err)
		return nil
	}
	s := consensus.TxSet{
		tr.ID(): tr,
	}
	id := s.ID()
	log.Println("onclose", tr.Hash(), "txsetid", hex.EncodeToString(id[:]))
	return s
}

// OnAccept is called when ledger is accepted by consensus
func (a *Adaptor) OnAccept(l *consensus.Ledger) {
	for h := range l.Txs {
		log.Println("onaccepted:", hex.EncodeToString(h[:]))
	}
	if len(l.Txs) == 0 {
		log.Println("no txs is onaccepted")
	}
	if err := Confirm(a.s, l); err != nil {
		log.Println(err)
		return
	}
	if err := PutLedger(a.s, l); err != nil {
		log.Println(err)
	}
}

// Propose proposes the position to Peers.
func (a *Adaptor) Propose(prop *consensus.Proposal) {
	id := prop.ID()
	adr, err := a.s.ValidatorAddress()
	if err != nil {
		log.Println(err)
		return
	}
	sig, err := adr.Sign(id[:])
	if err != nil {
		log.Println(err)
		return
	}
	prop.Signature = arypack.Marshal(sig)
	peer.BroadcastProposal(a.s, prop)
}

//SharePosition  shares a received Peer proposal with other Peer's.
func (a *Adaptor) SharePosition(prop *consensus.Proposal) {
	peer.BroadcastProposal(a.s, prop)
}

// ShareTx shares a disputed transaction with Peers
func (a *Adaptor) ShareTx(t consensus.TxT) {} //nothing

// ShareTxset Share given transaction set with Peers
func (a *Adaptor) ShareTxset(ts consensus.TxSet) {} //nothing

//ShareValidaton  shares my validation
func (a *Adaptor) ShareValidaton(v *consensus.Validation) {
	id := v.ID()
	adr, err := a.s.ValidatorAddress()
	if err != nil {
		log.Println(err)
		return
	}
	sig, err := adr.Sign(id[:])
	if err != nil {
		log.Println(err)
		return
	}
	v.Signature = arypack.Marshal(sig)
	peer.BroadcastValidatoin(a.s, v)
}

// ShouldAccept returns true if the result should be accepted
func (a *Adaptor) ShouldAccept(result *consensus.Result) bool {
	log.Println("#proposers", result.Proposers)
	if len(a.s.TrustedNodes)/2+1 > int(result.Proposers) {
		return false
	}
	return true
}
