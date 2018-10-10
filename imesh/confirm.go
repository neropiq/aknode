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
	"bytes"

	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/dgraph-io/badger"
)

func merge(base, refs map[[34]byte]tx.Hash, conflicts, conf map[[32]byte]struct{}) {
	for k := range conf {
		conflicts[k] = struct{}{}
	}
	for k, v := range refs {
		if h, ok := base[k]; !ok {
			base[k] = v
		} else {
			if bytes.Compare(v, h) == 0 {
				continue
			}
			if bytes.Compare(v, h) < 0 {
				base[k] = v
				conflicts[h.Array()] = struct{}{}
			} else {
				base[k] = h
				conflicts[v.Array()] = struct{}{}
			}
		}
	}
}

func checkConflict(s *setting.Setting, h tx.Hash, visited map[[32]byte]struct{}) (map[[34]byte]tx.Hash, map[[32]byte]struct{}, error) {
	base := make(map[[34]byte]tx.Hash)
	conflicts := make(map[[32]byte]struct{})
	if _, y := visited[h.Array()]; y {
		return base, conflicts, nil
	}
	ti, err := GetTxInfo(s.DB, h)
	if err != nil {
		return nil, nil, err
	}
	if ti.StatNo != StatusPending {
		return base, conflicts, nil
	}
	for _, p := range ti.Body.Parent {
		refs, conf, err := checkConflict(s, p, visited)
		if err != nil {
			return nil, nil, err
		}
		merge(base, refs, conflicts, conf)
	}
	for _, p := range ti.Body.Inputs {
		refs, conf, err := checkConflict(s, p.PreviousTX, visited)
		if err != nil {
			return nil, nil, err
		}
		merge(base, refs, conflicts, conf)
	}
	for _, p := range ti.Body.MultiSigIns {
		refs, conf, err := checkConflict(s, p.PreviousTX, visited)
		if err != nil {
			return nil, nil, err
		}
		merge(base, refs, conflicts, conf)
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		refs, conf, err := checkConflict(s, ticket, visited)
		if err != nil {
			return nil, nil, err
		}
		merge(base, refs, conflicts, conf)
	}
	for _, p := range ti.Body.Inputs {
		inout := tx.InoutHash{
			Hash:  p.PreviousTX,
			Type:  tx.TypeIn,
			Index: p.Index,
		}
		if _, ok := base[inout.Serialize()]; !ok {
			base[inout.Serialize()] = h
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		inout := tx.InoutHash{
			Hash:  p.PreviousTX,
			Type:  tx.TypeMulin,
			Index: p.Index,
		}
		if _, ok := base[inout.Serialize()]; !ok {
			base[inout.Serialize()] = h
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		inout := tx.InoutHash{
			Hash:  ticket,
			Type:  tx.TypeTicketin,
			Index: 0,
		}
		if _, ok := base[inout.Serialize()]; !ok {
			base[inout.Serialize()] = h
		}
	}
	visited[h.Array()] = struct{}{}
	return base, conflicts, nil
}

func confirm(s *setting.Setting, txn *badger.Txn, rejectTxs map[[32]byte]struct{}, h tx.Hash, no [32]byte) error {
	var ti TxInfo
	if err := db.Get(txn, h, &ti, db.HeaderTxInfo); err != nil {
		return err
	}
	if ti.StatNo != StatusPending {
		return nil
	}
	for _, p := range ti.Body.Parent {
		if err := confirm(s, txn, rejectTxs, p, no); err != nil {
			return err
		}
	}
	for _, p := range ti.Body.Inputs {
		if err := confirm(s, txn, rejectTxs, p.PreviousTX, no); err != nil {
			return err
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		if err := confirm(s, txn, rejectTxs, p.PreviousTX, no); err != nil {
			return err
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		if err := confirm(s, txn, rejectTxs, ticket, no); err != nil {
			return err
		}
	}
	reject := false
	if _, ok := rejectTxs[h.Array()]; ok {
		reject = true
	}
	for _, p := range ti.Body.Inputs {
		var pti TxInfo
		if err := db.Get(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return err
		}
		if !pti.IsAccepted() {
			reject = true
		}
		if pti.OutputStatus[0][p.Index].IsSpent {
			reject = true
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		var pti TxInfo
		if err := db.Get(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return err
		}
		if !pti.IsAccepted() {
			reject = true
		}
		if pti.OutputStatus[1][p.Index].IsSpent {
			reject = true
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		var pti TxInfo
		if err := db.Get(txn, ticket, &pti, db.HeaderTxInfo); err != nil {
			return err
		}
		if !pti.IsAccepted() {
			reject = true
		}
		if pti.OutputStatus[2][0].IsSpent {
			reject = true
		}
	}
	ti.StatNo = no
	if reject {
		ti.IsRejected = true
		return db.Put(txn, h, ti, db.HeaderTxInfo)
	}
	if err := db.Put(txn, h, ti, db.HeaderTxInfo); err != nil {
		return err
	}
	for _, p := range ti.Body.Inputs {
		var pti TxInfo
		if err := db.Get(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return err
		}
		pti.OutputStatus[0][p.Index].IsSpent = true
		if err := db.Put(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		var pti TxInfo
		if err := db.Get(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return err
		}
		pti.OutputStatus[1][p.Index].IsSpent = true
		if err := db.Put(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		var pti TxInfo
		if err := db.Get(txn, ticket, &pti, db.HeaderTxInfo); err != nil {
			return err
		}
		pti.OutputStatus[2][0].IsSpent = true
		if err := db.Put(txn, ticket, &pti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	return nil
}

//Confirm txs from h.
func Confirm(s *setting.Setting, h tx.Hash, no [32]byte) error {
	mutex.Lock()
	defer mutex.Unlock()
	visited := make(map[[32]byte]struct{})
	_, conflicts, err := checkConflict(s, h, visited)
	if err != nil {
		return err
	}
	return s.DB.Update(func(txn *badger.Txn) error {
		return confirm(s, txn, conflicts, h, no)
	})
}

//RevertConfirmation reverts confirmation from h.
func RevertConfirmation(s *setting.Setting, h tx.Hash, no StatNo) error {
	mutex.Lock()
	defer mutex.Unlock()
	return s.DB.Update(func(txn *badger.Txn) error {
		return revertConfirmation(s, txn, h, no)
	})
}

func revertConfirmation(s *setting.Setting, txn *badger.Txn, h tx.Hash, no StatNo) error {
	var ti TxInfo
	if err := db.Get(txn, h, &ti, db.HeaderTxInfo); err != nil {
		return err
	}
	if ti.StatNo != no {
		return nil
	}
	for _, p := range ti.Body.Parent {
		if err := revertConfirmation(s, txn, p, no); err != nil {
			return err
		}
	}
	for _, p := range ti.Body.Inputs {
		if err := revertConfirmation(s, txn, p.PreviousTX, no); err != nil {
			return err
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		if err := revertConfirmation(s, txn, p.PreviousTX, no); err != nil {
			return err
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		if err := revertConfirmation(s, txn, ticket, no); err != nil {
			return err
		}
	}
	ti.StatNo = StatusPending
	if err := db.Put(txn, h, ti, db.HeaderTxInfo); err != nil {
		return err
	}
	for _, p := range ti.Body.Inputs {
		var pti TxInfo
		if err := db.Get(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return err
		}
		pti.OutputStatus[0][p.Index].IsSpent = false
		if err := db.Put(txn, p.PreviousTX, pti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		var pti TxInfo
		if err := db.Get(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return err
		}
		pti.OutputStatus[1][p.Index].IsSpent = false
		if err := db.Put(txn, p.PreviousTX, pti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		var pti TxInfo
		if err := db.Get(txn, ticket, &pti, db.HeaderTxInfo); err != nil {
			return err
		}
		pti.OutputStatus[2][0].IsSpent = false
		if err := db.Put(txn, ticket, pti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	return nil
}
