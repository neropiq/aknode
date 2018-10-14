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
			if bytes.Equal(v, h) {
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

func confirm(s *setting.Setting, txn *badger.Txn, rejectTxs map[[32]byte]struct{}, h tx.Hash, no [32]byte) ([]tx.Hash, error) {
	var ti TxInfo
	var hs []tx.Hash
	if err := db.Get(txn, h, &ti, db.HeaderTxInfo); err != nil {
		return nil, err
	}
	if ti.StatNo != StatusPending {
		return hs, nil
	}
	for _, p := range ti.Body.Parent {
		hs2, err := confirm(s, txn, rejectTxs, p, no)
		if err != nil {
			return nil, err
		}
		hs = append(hs, hs2...)
	}
	for _, p := range ti.Body.Inputs {
		hs2, err := confirm(s, txn, rejectTxs, p.PreviousTX, no)
		if err != nil {
			return nil, err
		}
		hs = append(hs, hs2...)
	}
	for _, p := range ti.Body.MultiSigIns {
		hs2, err := confirm(s, txn, rejectTxs, p.PreviousTX, no)
		if err != nil {
			return nil, err
		}
		hs = append(hs, hs2...)
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		hs2, err := confirm(s, txn, rejectTxs, ticket, no)
		if err != nil {
			return nil, err
		}
		hs = append(hs, hs2...)
	}
	reject := false
	if _, ok := rejectTxs[h.Array()]; ok {
		reject = true
	}
	for _, p := range ti.Body.Inputs {
		var pti TxInfo
		if err := db.Get(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return nil, err
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
			return nil, err
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
			return nil, err
		}
		if !pti.IsAccepted() {
			reject = true
		}
		if pti.OutputStatus[2][0].IsSpent {
			reject = true
		}
	}
	ti.StatNo = no
	hs = append(hs, h)
	if reject {
		ti.IsRejected = true
		return hs, db.Put(txn, h, ti, db.HeaderTxInfo)
	}
	if err := db.Put(txn, h, ti, db.HeaderTxInfo); err != nil {
		return nil, err
	}
	for _, p := range ti.Body.Inputs {
		var pti TxInfo
		if err := db.Get(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return nil, err
		}
		pti.OutputStatus[0][p.Index].IsSpent = true
		if err := db.Put(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return nil, err
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		var pti TxInfo
		if err := db.Get(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return nil, err
		}
		pti.OutputStatus[1][p.Index].IsSpent = true
		if err := db.Put(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return nil, err
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		var pti TxInfo
		if err := db.Get(txn, ticket, &pti, db.HeaderTxInfo); err != nil {
			return nil, err
		}
		pti.OutputStatus[2][0].IsSpent = true
		if err := db.Put(txn, ticket, &pti, db.HeaderTxInfo); err != nil {
			return nil, err
		}
	}
	return hs, nil
}

//Confirm txs from h.
func Confirm(s *setting.Setting, h tx.Hash, no [32]byte) ([]tx.Hash, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var hs []tx.Hash
	visited := make(map[[32]byte]struct{})
	_, conflicts, err := checkConflict(s, h, visited)
	if err != nil {
		return nil, err
	}
	err = s.DB.Update(func(txn *badger.Txn) error {
		var err2 error
		hs, err2 = confirm(s, txn, conflicts, h, no)
		return err2
	})
	return hs, err
}

//RevertConfirmation reverts confirmation from h.
func RevertConfirmation(s *setting.Setting, h tx.Hash, no StatNo) ([]tx.Hash, error) {
	mutex.Lock()
	defer mutex.Unlock()
	var hs []tx.Hash
	err := s.DB.Update(func(txn *badger.Txn) error {
		var err2 error
		hs, err2 = revertConfirmation(s, txn, h, no)
		return err2
	})
	return hs, err
}

func revertConfirmation(s *setting.Setting, txn *badger.Txn, h tx.Hash, no StatNo) ([]tx.Hash, error) {
	var ti TxInfo
	var hs []tx.Hash
	if err := db.Get(txn, h, &ti, db.HeaderTxInfo); err != nil {
		return nil, err
	}
	if ti.StatNo != no {
		return hs, nil
	}
	hs = append(hs, h)
	for _, p := range ti.Body.Parent {
		hs2, err := revertConfirmation(s, txn, p, no)
		if err != nil {
			return nil, err
		}
		hs = append(hs, hs2...)
	}
	for _, p := range ti.Body.Inputs {
		hs2, err := revertConfirmation(s, txn, p.PreviousTX, no)
		if err != nil {
			return nil, err
		}
		hs = append(hs, hs2...)
	}
	for _, p := range ti.Body.MultiSigIns {
		hs2, err := revertConfirmation(s, txn, p.PreviousTX, no)
		if err != nil {
			return nil, err
		}
		hs = append(hs, hs2...)
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		hs2, err := revertConfirmation(s, txn, ticket, no)
		if err != nil {
			return nil, err
		}
		hs = append(hs, hs2...)
	}
	rejected := ti.IsRejected
	ti.StatNo = StatusPending
	ti.IsRejected = false
	if err := db.Put(txn, h, ti, db.HeaderTxInfo); err != nil {
		return nil, err
	}
	if rejected {
		return hs, nil
	}
	for _, p := range ti.Body.Inputs {
		var pti TxInfo
		if err := db.Get(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return nil, err
		}
		pti.OutputStatus[0][p.Index].IsSpent = false
		if err := db.Put(txn, p.PreviousTX, pti, db.HeaderTxInfo); err != nil {
			return nil, err
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		var pti TxInfo
		if err := db.Get(txn, p.PreviousTX, &pti, db.HeaderTxInfo); err != nil {
			return nil, err
		}
		pti.OutputStatus[1][p.Index].IsSpent = false
		if err := db.Put(txn, p.PreviousTX, pti, db.HeaderTxInfo); err != nil {
			return nil, err
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		var pti TxInfo
		if err := db.Get(txn, ticket, &pti, db.HeaderTxInfo); err != nil {
			return nil, err
		}
		pti.OutputStatus[2][0].IsSpent = false
		if err := db.Put(txn, ticket, pti, db.HeaderTxInfo); err != nil {
			return nil, err
		}
	}
	return hs, nil
}
