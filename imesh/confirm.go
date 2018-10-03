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
	"sort"

	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/setting"
	"github.com/dgraph-io/badger"
)

func checkConflict(s *setting.Setting, conflicts map[[34]byte][]tx.Hash, h tx.Hash) error {
	ti, err := GetTxInfo(s.DB, h)
	if err != nil {
		return err
	}
	if ti.StatNo != StatusPending {
		return nil
	}
	for _, p := range ti.Body.Parent {
		if err := checkConflict(s, conflicts, p); err != nil {
			return err
		}
	}
	for _, p := range ti.Body.Inputs {
		if err := checkConflict(s, conflicts, p.PreviousTX); err != nil {
			return err
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		if err := checkConflict(s, conflicts, p.PreviousTX); err != nil {
			return err
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		if err := checkConflict(s, conflicts, ticket); err != nil {
			return err
		}
	}
	for _, p := range ti.Body.Inputs {
		inout := tx.InoutHash{
			Hash:  p.PreviousTX,
			Type:  tx.TypeIn,
			Index: p.Index,
		}
		if len(conflicts[inout.Serialize()]) == 0 {
			conflicts[inout.Serialize()] = append(conflicts[inout.Serialize()], p.PreviousTX)
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		inout := tx.InoutHash{
			Hash:  p.PreviousTX,
			Type:  tx.TypeMulin,
			Index: p.Index,
		}
		if len(conflicts[inout.Serialize()]) == 0 {
			conflicts[inout.Serialize()] = append(conflicts[inout.Serialize()], p.PreviousTX)
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		inout := tx.InoutHash{
			Hash:  ticket,
			Type:  tx.TypeTicketin,
			Index: 0,
		}
		if len(conflicts[inout.Serialize()]) == 0 {
			conflicts[inout.Serialize()] = append(conflicts[inout.Serialize()], ticket)
		}
	}
	return nil
}

func getRejectTxs(conflicts map[[34]byte][]tx.Hash) map[[32]byte]struct{} {
	rejectTx := make(map[[32]byte]struct{})
	for {
		var confTx []tx.Hash
		exists := make(map[[32]byte]struct{})
		for k, hs := range conflicts {
			if len(hs) <= 1 {
				delete(conflicts, k)
				continue
			}
			for _, h := range hs {
				if _, ok := exists[h.Array()]; !ok {
					exists[h.Array()] = struct{}{}
					confTx = append(confTx, h)
				}
			}
		}
		if len(confTx) == 0 {
			break
		}
		sort.Slice(confTx, func(i, j int) bool {
			return bytes.Compare(confTx[i], confTx[j]) > 0
		})
		rejectTx[confTx[0].Array()] = struct{}{}
	loop:
		for k, hs := range conflicts {
			for i, h := range hs {
				if bytes.Equal(h, confTx[0]) {
					conflicts[k] = append(conflicts[k][:i], conflicts[k][i+1:]...)
					continue loop
				}
			}
		}
	}
	return rejectTx
}

func confirm(s *setting.Setting, txn *badger.Txn,
	rejectTxs map[[32]byte]struct{}, h tx.Hash, no [32]byte) error {
	ti, err := GetTxInfo(s.DB, h)
	if err != nil {
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
		pti, err := GetTxInfo(s.DB, p.PreviousTX)
		if err != nil {
			return err
		}
		if pti.OutputStatus[0][p.Index].IsSpent {
			reject = true
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		pti, err := GetTxInfo(s.DB, p.PreviousTX)
		if err != nil {
			return err
		}
		if pti.OutputStatus[1][p.Index].IsSpent {
			reject = true
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		pti, err := GetTxInfo(s.DB, ticket)
		if err != nil {
			return err
		}
		if pti.OutputStatus[2][0].IsSpent {
			reject = true
		}
	}
	if reject {
		ti.StatNo = StatusRejected
		return db.Put(txn, h, ti, db.HeaderTxInfo)
	}
	ti.StatNo = no
	if err := db.Put(txn, h, ti, db.HeaderTxInfo); err != nil {
		return err
	}
	for _, p := range ti.Body.Inputs {
		pti, err := GetTxInfo(s.DB, p.PreviousTX)
		if err != nil {
			return err
		}
		pti.OutputStatus[0][p.Index].IsSpent = true
		if err := db.Put(txn, p.PreviousTX, pti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		pti, err := GetTxInfo(s.DB, p.PreviousTX)
		if err != nil {
			return err
		}
		pti.OutputStatus[1][p.Index].IsSpent = true
		if err := db.Put(txn, p.PreviousTX, pti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		pti, err := GetTxInfo(s.DB, ticket)
		if err != nil {
			return err
		}
		pti.OutputStatus[2][0].IsSpent = true
		if err := db.Put(txn, ticket, pti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	return nil
}

//Confirm txs from h.
func Confirm(s *setting.Setting, h tx.Hash, no [32]byte) error {
	conflicts := make(map[[34]byte][]tx.Hash)
	if err := checkConflict(s, conflicts, h); err != nil {
		return err
	}
	rejectTx := getRejectTxs(conflicts)
	return s.DB.Update(func(txn *badger.Txn) error {
		return confirm(s, txn, rejectTx, h, no)
	})
}

//RevertConfirmation reverts confirmation from h.
func RevertConfirmation(s *setting.Setting, h tx.Hash, no StatNo) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return revertConfirmation(s, txn, h, no)
	})
}

func revertConfirmation(s *setting.Setting, txn *badger.Txn, h tx.Hash, no StatNo) error {
	ti, err := GetTxInfo(s.DB, h)
	if err != nil {
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
		pti, err := GetTxInfo(s.DB, p.PreviousTX)
		if err != nil {
			return err
		}
		pti.OutputStatus[0][p.Index].IsSpent = false
		if err := db.Put(txn, p.PreviousTX, pti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	for _, p := range ti.Body.MultiSigIns {
		pti, err := GetTxInfo(s.DB, p.PreviousTX)
		if err != nil {
			return err
		}
		pti.OutputStatus[1][p.Index].IsSpent = false
		if err := db.Put(txn, p.PreviousTX, pti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	if ticket := ti.Body.TicketInput; ticket != nil {
		pti, err := GetTxInfo(s.DB, ticket)
		if err != nil {
			return err
		}
		pti.OutputStatus[2][0].IsSpent = false
		if err := db.Put(txn, ticket, pti, db.HeaderTxInfo); err != nil {
			return err
		}
	}
	return nil
}
