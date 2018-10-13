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
	"log"
	"os"
	"testing"
	"time"

	"github.com/AidosKuneen/consensus"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/setting"
)

var s setting.Setting
var a, b, c, d *address.Address
var genesis []tx.Hash

func setup(t *testing.T) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	var err2 error
	if err := os.RemoveAll("./test_db"); err != nil {
		log.Println(err)
	}
	s.DB, err2 = db.Open("./test_db")
	if err2 != nil {
		panic(err2)
	}
	s.Config = aklib.DebugConfig
	seed := address.GenerateSeed32()
	a, err2 = address.New(s.Config, seed)
	if err2 != nil {
		t.Error(err2)
	}
	s.Config.Genesis = map[string]uint64{
		a.Address58(s.Config): aklib.ADKSupply,
	}
	seed = address.GenerateSeed32()
	b, err2 = address.New(s.Config, seed)
	if err2 != nil {
		t.Error(err2)
	}
	seed = address.GenerateSeed32()
	c, err2 = address.New(s.Config, seed)
	if err2 != nil {
		t.Error(err2)
	}
	seed = address.GenerateSeed32()
	d, err2 = address.New(s.Config, seed)
	if err2 != nil {
		t.Error(err2)
	}
	leaves.Init(&s)
	if err := imesh.Init(&s); err != nil {
		t.Error(err)
	}
	genesis = leaves.Get(1)
	if len(genesis) != 1 {
		t.Error("invalid genesis")
	}
}

func teardown(t *testing.T) {
	if err := os.RemoveAll("./test_db"); err != nil {
		t.Error(err)
	}
}
func TestConsensus(t *testing.T) {
	setup(t)
	defer teardown(t)
	ch := make(chan []tx.Hash)
	RegisterTxNotifier(ch)

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
	if err := imesh.CheckAddTx(&s, trs[0], tx.TypeNormal); err != nil {
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
	if err := imesh.CheckAddTx(&s, trs[1], tx.TypeNormal); err != nil {
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
	if err := imesh.CheckAddTx(&s, trs[2], tx.TypeNormal); err != nil {
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
	if err := imesh.CheckAddTx(&s, trs[3], tx.TypeNormal); err != nil {
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
	if err := imesh.CheckAddTx(&s, trs[4], tx.TypeNormal); err != nil {
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
	if err := imesh.CheckAddTx(&s, trs[5], tx.TypeNormal); err != nil {
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
	if err := imesh.CheckAddTx(&s, trs[6], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	trs[7] = tx.New(s.Config, trs[5].Hash(), trs[6].Hash())
	if err := trs[7].PoW(); err != nil {
		t.Error(err)
	}
	if err := imesh.CheckAddTx(&s, trs[7], tx.TypeNormal); err != nil {
		t.Error(err)
	}

	if _, err := imesh.Resolve(&s); err != nil {
		t.Error(err)
	}
	for _, tr := range trs {
		log.Println(tr.Hash())
	}
	var id [32]byte
	id[0] = 42
	conf := 5
	rej := 6
	if bytes.Compare(trs[conf].Hash(), trs[rej].Hash()) > 0 {
		conf = 6
		rej = 5
	}
	go func() {
		var tr []tx.Hash
		select {
		case tr = <-ch: //l3
		case <-time.Tick(10 * time.Second):
			t.Fatal("failed to notify")
		}
		if len(tr) != 4 {
			t.Error("invalid accepted txs")
		}
		for _, i := range []int{0, 1, 2, 3} {
			ok := false
			for _, h := range tr {
				if bytes.Equal(h, trs[i].Hash()) {
					ok = true
				}
			}
			if !ok {
				t.Error("invalid accepted txs")
			}
		}
		select {
		case tr = <-ch: //l7
		case <-time.Tick(10 * time.Second):
			t.Fatal("failed to notify")
		}
		if len(tr) != 2 {
			t.Error("invalid accepted txs", len(tr))
		}
		for _, i := range []int{conf, 7} {
			ok := false
			for _, h := range tr {
				if bytes.Equal(h, trs[i].Hash()) {
					ok = true
				}
			}
			if !ok {
				t.Error("invalid accepted txs")
			}
		}
		select {
		case tr = <-ch: //l6
		case <-time.Tick(10 * time.Second):
			t.Fatal("failed to notify")
		}
		if len(tr) != 1 {
			t.Error("invalid accepted txs", len(tr))
		}
		for _, i := range []int{6} {
			ok := false
			for _, h := range tr {
				if bytes.Equal(h, trs[i].Hash()) {
					ok = true
				}
			}
			if !ok {
				t.Error("invalid accepted txs")
			}
		}
	}()

	l3 := &consensus.Ledger{
		ParentID: consensus.GenesisID,
		Seq:      1,
		Txs: consensus.TxSet{
			trs[3].ID(): trs[3],
		},
	}
	l3.IndexOf = func(s consensus.Seq) consensus.LedgerID {
		switch s {
		case 0:
			return consensus.GenesisID
		case 1:
			return l3.ID()
		}
		panic("invalid indexof")
	}
	l7 := &consensus.Ledger{
		ParentID: l3.ID(),
		Seq:      2,
		Txs: consensus.TxSet{
			trs[7].ID(): trs[7],
		},
	}
	l7.IndexOf = func(s consensus.Seq) consensus.LedgerID {
		switch s {
		case 0:
			return consensus.GenesisID
		case 1:
			return l3.ID()
		case 2:
			return l7.ID()
		}
		panic("invalid indexof")
	}
	l6 := &consensus.Ledger{
		ParentID: l3.ID(),
		Seq:      2,
		Txs: consensus.TxSet{
			trs[6].ID(): trs[6],
		},
	}
	l6.IndexOf = func(s consensus.Seq) consensus.LedgerID {
		switch s {
		case 0:
			return consensus.GenesisID
		case 1:
			return l3.ID()
		case 2:
			return l6.ID()
		}
		panic("invalid indexof")
	}

	if err := Confirm(&s, l3); err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{0, 1, 2, 3} {
		tr, err := imesh.GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if !tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo != imesh.StatNo(l3.ID()) || tr.StatNo == imesh.StatusPending || tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}

	if err := Confirm(&s, l7); err != nil {
		t.Error(err)
	}
	for _, i := range []int{conf, 7} {
		tr, err := imesh.GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if !tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo != imesh.StatNo(l7.ID()) || tr.StatNo == imesh.StatusPending || tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}
	for _, i := range []int{4, rej} {
		tr, err := imesh.GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo != imesh.StatNo(l7.ID()) || tr.StatNo == imesh.StatusPending || !tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}

	if err := Confirm(&s, l6); err != nil {
		t.Error(err)
	}
	for _, i := range []int{0, 1, 2, 3} {
		tr, err := imesh.GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if !tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo != imesh.StatNo(l3.ID()) || tr.StatNo == imesh.StatusPending || tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}
	for _, i := range []int{4} {
		tr, err := imesh.GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo != imesh.StatNo(l6.ID()) || tr.StatNo == imesh.StatusPending || !tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}
	for _, i := range []int{6} {
		tr, err := imesh.GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if !tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo != imesh.StatNo(l6.ID()) || tr.StatNo == imesh.StatusPending || tr.IsRejected {
			t.Error("invalid Cofnrim", i)
		}
	}
	for _, i := range []int{5, 7} {
		tr, err := imesh.GetTxInfo(s.DB, trs[i].Hash())
		if err != nil {
			t.Error(err)
		}
		if tr.IsAccepted() {
			t.Error("invalid Cofnrim", i)
		}
		if tr.StatNo != imesh.StatusPending {
			t.Error("invalid Cofnrim", i)
		}
	}

}
