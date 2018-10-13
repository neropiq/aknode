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
	"log"
	"testing"
	"time"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/tx"
)

func TestConfirm(t *testing.T) {
	setup(t)
	defer teardown(t)

	var trs [8]*tx.Transaction

	trs[0] = tx.New(s.Config, genesis[0])
	trs[0].AddInput(genesis[0], 0)
	if err := trs[0].AddOutput(s.Config, a.Address58(s.Config), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := trs[0].Sign(a); err != nil {
		t.Error(err)
	}
	if err := trs[0].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs[0], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	trs[1] = tx.New(s.Config, genesis[0])
	trs[1].AddInput(trs[0].Hash(), 0)
	if err := trs[1].AddOutput(s.Config, b.Address58(s.Config), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := trs[1].Sign(a); err != nil {
		t.Error(err)
	}
	if err := trs[1].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs[1], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	trs[2] = tx.New(s.Config, trs[1].Hash())
	trs[2].AddInput(trs[1].Hash(), 0)
	if err := trs[2].AddOutput(s.Config, b.Address58(s.Config), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := trs[2].Sign(b); err != nil {
		t.Error(err)
	}
	if err := trs[2].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs[2], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	trs[3] = tx.New(s.Config, trs[2].Hash())
	trs[3].AddInput(trs[2].Hash(), 0)
	if err := trs[3].AddOutput(s.Config, c.Address58(s.Config), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := trs[3].Sign(b); err != nil {
		t.Error(err)
	}
	if err := trs[3].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs[3], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	//double spend
	trs[4] = tx.New(s.Config, trs[3].Hash())
	trs[4].AddInput(trs[2].Hash(), 0)
	if err := trs[4].AddOutput(s.Config, b.Address58(s.Config), 200); err != nil {
		t.Error(err)
	}
	if err := trs[4].AddOutput(s.Config, a.Address58(s.Config), aklib.ADKSupply-200); err != nil {
		t.Error(err)
	}
	if err := trs[4].Sign(b); err != nil {
		t.Error(err)
	}
	if err := trs[4].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs[4], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	//one of double spend
	trs[5] = tx.New(s.Config, trs[4].Hash())
	trs[5].AddInput(trs[3].Hash(), 0)
	if err := trs[5].AddOutput(s.Config, d.Address58(s.Config), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := trs[5].Sign(c); err != nil {
		t.Error(err)
	}
	if err := trs[5].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs[5], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	//one of double spend
	trs[6] = tx.New(s.Config, trs[4].Hash())
	trs[6].AddInput(trs[3].Hash(), 0)
	if err := trs[6].AddOutput(s.Config, d.Address58(s.Config), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := trs[6].Sign(c); err != nil {
		t.Error(err)
	}
	if err := trs[6].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs[6], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	trs[7] = tx.New(s.Config, trs[5].Hash(), trs[6].Hash())
	if err := trs[7].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs[7], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	if _, err := Resolve(&s); err != nil {
		t.Error(err)
	}
	for _, tr := range trs {
		log.Println(tr.Hash())
	}
	var id [32]byte
	id[0] = 42
	hs, err := Confirm(&s, trs[7].Hash(), id)
	if err != nil {
		t.Error(err)
	}
	conf := 5
	rej := 6
	if bytes.Compare(trs[conf].Hash(), trs[rej].Hash()) > 0 {
		conf = 6
		rej = 5
	}
	if len(hs) != 8 {
		for _, h := range hs {
			t.Log(h)
		}
		t.Error("invalid accepted txs", len(hs))
	}
	for _, i := range []int{0, 1, 2, 3, conf, 7} {
		ok := false
		for _, h := range hs {
			if bytes.Equal(h, trs[i].Hash()) {
				ok = true
			}
		}
		if !ok {
			t.Error("invalid accepted txs")
		}
		tr, err := GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if !tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo[0] != 42 || tr.StatNo == StatusPending || tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}
	for _, i := range []int{4, rej} {
		ok := false
		for _, h := range hs {
			if bytes.Equal(h, trs[i].Hash()) {
				ok = true
			}
		}
		if !ok {
			t.Error("invalid accepted txs")
		}
		tr, err := GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo[0] != 42 || tr.StatNo == StatusPending || !tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}

	//another confirmation
	var trs2 [6]*tx.Transaction

	//spend a rejected tx
	trs2[0] = tx.New(s.Config, genesis[0])
	trs2[0].AddInput(trs[4].Hash(), 0)
	if err := trs2[0].AddOutput(s.Config, a.Address58(s.Config), 200); err != nil {
		t.Error(err)
	}
	if err := trs2[0].Sign(b); err != nil {
		t.Error(err)
	}
	if err := trs2[0].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs2[0], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	//ok
	trs2[1] = tx.New(s.Config, trs[0].Hash())
	trs2[1].AddInput(trs[conf].Hash(), 0)
	if err := trs2[1].AddOutput(s.Config, a.Address58(s.Config), 100); err != nil {
		t.Error(err)
	}
	if err := trs2[1].AddOutput(s.Config, b.Address58(s.Config), aklib.ADKSupply-100); err != nil {
		t.Error(err)
	}
	if err := trs2[1].Sign(d); err != nil {
		t.Error(err)
	}
	if err := trs2[1].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs2[1], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	//one of input is double spend
	trs2[2] = tx.New(s.Config, genesis[0])
	trs2[2].AddInput(trs[4].Hash(), 0)
	trs2[2].AddInput(trs2[1].Hash(), 0)
	if err := trs2[2].AddOutput(s.Config, b.Address58(s.Config), 200+100); err != nil {
		t.Error(err)
	}
	if err := trs2[2].Sign(b); err != nil {
		t.Error(err)
	}
	if err := trs2[2].Sign(a); err != nil {
		t.Error(err)
	}
	if err := trs2[2].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs2[2], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	//ok
	trs2[3] = tx.New(s.Config, trs2[2].Hash())
	trs2[3].AddInput(trs2[1].Hash(), 0)
	if err := trs2[3].AddOutput(s.Config, c.Address58(s.Config), 100); err != nil {
		t.Error(err)
	}
	if err := trs2[3].Sign(a); err != nil {
		t.Error(err)
	}
	if err := trs2[3].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs2[3], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	//ok
	trs2[4] = tx.New(s.Config, trs[3].Hash())
	trs2[4].AddInput(trs2[1].Hash(), 1)
	trs2[4].AddInput(trs2[3].Hash(), 0)
	if err := trs2[4].AddOutput(s.Config, b.Address58(s.Config), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := trs2[4].Sign(c); err != nil {
		t.Error(err)
	}
	if err := trs2[4].Sign(b); err != nil {
		t.Error(err)
	}
	if err := trs2[4].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs2[4], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	trs2[5] = tx.New(s.Config, trs[5].Hash(), trs2[4].Hash(), trs2[3].Hash(), trs2[0].Hash())
	if err := trs2[5].PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, trs2[5], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	if _, err := Resolve(&s); err != nil {
		t.Error(err)
	}
	time.Sleep(6 * time.Second)
	for _, tr := range trs2 {
		log.Println(tr.Hash())
	}
	id[0] = 43
	hs, err = Confirm(&s, trs2[5].Hash(), id)
	if err != nil {
		t.Error(err)
	}
	if len(hs) != 6 {
		t.Error("invalid accepted txs")
	}

	for _, i := range []int{0, 1, 2, 3, conf} {
		ok := false
		for _, h := range hs {
			if bytes.Equal(h, trs[i].Hash()) {
				ok = true
			}
		}
		if ok {
			t.Error("invalid accepted txs")
		}
		tr, err := GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if !tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo[0] != 42 || tr.StatNo == StatusPending || tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}
	for _, i := range []int{4, rej} {
		ok := false
		for _, h := range hs {
			if bytes.Equal(h, trs[i].Hash()) {
				ok = true
			}
		}
		if ok {
			t.Error("invalid accepted txs")
		}
		tr, err := GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo[0] != 42 || tr.StatNo == StatusPending || !tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}

	for _, i := range []int{1, 3, 4, 5} {
		ok := false
		for _, h := range hs {
			if bytes.Equal(h, trs2[i].Hash()) {
				ok = true
			}
		}
		if !ok {
			t.Error("invalid accepted txs")
		}
		tr, err := GetTxInfo(s.DB, trs2[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if !tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo[0] != 43 || tr.StatNo == StatusPending || tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}
	for _, i := range []int{0, 2} {
		ok := false
		for _, h := range hs {
			if bytes.Equal(h, trs2[i].Hash()) {
				ok = true
			}
		}
		if !ok {
			t.Error("invalid accepted txs")
		}
		tr, err := GetTxInfo(s.DB, trs2[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo[0] != 43 || tr.StatNo == StatusPending || !tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}

	hs, err = RevertConfirmation(&s, trs2[5].Hash(), id)
	if err != nil {
		t.Error(err)
	}
	if len(hs) != 6 {
		for _, h := range hs {
			t.Log(h)
		}
		t.Error("invalid reverted txs", len(hs))
	}

	for _, i := range []int{0, 1, 2, 3, conf} {
		ok := false
		for _, h := range hs {
			if bytes.Equal(h, trs[i].Hash()) {
				ok = true
			}
		}
		if ok {
			t.Error("invalid accepted txs")
		}
		tr, err := GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if !tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo[0] != 42 || tr.StatNo == StatusPending || tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}
	for _, i := range []int{4, rej} {
		ok := false
		for _, h := range hs {
			if bytes.Equal(h, trs[i].Hash()) {
				ok = true
			}
		}
		if ok {
			t.Error("invalid accepted txs")
		}
		tr, err := GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo[0] != 42 || tr.StatNo == StatusPending || !tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}
	for i, tr := range trs2 {
		ok := false
		for _, h := range hs {
			if bytes.Equal(h, trs2[i].Hash()) {
				ok = true
			}
		}
		if !ok {
			t.Error("invalid accepted txs")
		}
		tr, err := GetTxInfo(s.DB, tr.Hash())
		if err != nil {
			t.Error(err)
		}
		if tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo != StatusPending {
			t.Error("invalid revert", i)
		}
	}
}
