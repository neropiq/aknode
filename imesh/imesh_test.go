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
	"encoding/hex"
	"log"
	"os"
	"testing"
	"time"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"

	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/setting"
)

var s setting.Setting
var a *address.Address
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
	seed := address.GenerateSeed()
	a, err2 = address.New(address.Height10, seed, s.Config)
	if err2 != nil {
		t.Error(err2)
	}
	s.Config.Genesis = map[string]uint64{
		a.Address58(): aklib.ADKSupply,
	}
	leaves.Init(&s)
	if err := Init(&s); err != nil {
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

func TestImesh(t *testing.T) {
	setup(t)
	defer teardown(t)
	g, err2 := GetTx(&s, genesis[0])
	if err2 != nil {
		t.Error(err2)
	}
	if !bytes.Equal(g.Hash(), genesis[0]) {
		t.Error("should be equal")
	}
	t.Log(hex.EncodeToString(genesis[0]))
	tr := tx.New(s.Config, genesis[0])
	tr.AddInput(genesis[0], 0)
	if err := tr.AddOutput(s.Config, a.Address58(), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(a); err != nil {
		t.Error(err)
	}

	if err := CheckAddTx(&s, tr, tx.TxNormal); err == nil {
		t.Error("should be error")
	}
	if err := tr.PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, tr, tx.TxNormal); err != nil {
		t.Error(err)
	}
	trs, err := Resolve(&s)
	if err != nil {
		t.Error(err)
	}
	if len(trs) != 1 {
		t.Error("invalid resolved txs")
	}
	if !bytes.Equal(trs[0].Hash, tr.Hash()) || trs[0].Type != tx.TxNormal {
		t.Error("invalid resolved tx")
	}

	tr0 := tx.New(s.Config, genesis[0])
	tr0.AddInput(genesis[0], 0)
	if err := tr0.AddOutput(s.Config, a.Address58(), aklib.ADKSupply-1); err != nil {
		t.Error(err)
	}
	if err := tr0.Sign(a); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, tr0, tx.TxNormal); err == nil {
		t.Error("should be error")
	}
	if err := tr0.PoW(); err != nil {
		t.Error(err)
	}
	t.Log(hex.EncodeToString(tr0.Hash()))
	if err := CheckAddTx(&s, tr0, tx.TxNormal); err != nil {
		t.Error(err)
	}
	trs, err2 = Resolve(&s)
	if err2 != nil {
		t.Error(err2)
	}
	if len(trs) != 0 {
		t.Error("invalid resolved tx")
	}

	tr1 := tx.New(s.Config, tr.Hash())
	tr1.AddInput(genesis[0], 0)
	if err := tr1.AddOutput(s.Config, a.Address58(), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tr1.Sign(a); err != nil {
		t.Error(err)
	}
	if err := tr1.PoW(); err != nil {
		t.Error(err)
	}

	tr2 := tx.New(s.Config, genesis[0])
	tr2.AddInput(tr1.Hash(), 0)
	if err := tr2.AddOutput(s.Config, a.Address58(), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tr2.Sign(a); err != nil {
		t.Error(err)
	}
	if err := tr2.PoW(); err != nil {
		t.Error(err)
	}
	t.Log(hex.EncodeToString(tr1.Hash()))
	t.Log(hex.EncodeToString(tr2.Hash()))
	t.Log(hex.EncodeToString(genesis[0]))
	if err := CheckAddTx(&s, tr2, tx.TxNormal); err != nil {
		t.Error(err)
	}
	trs, err2 = Resolve(&s)
	if err2 != nil {
		t.Error(err2)
	}
	if len(trs) != 0 {
		t.Error("invalid resolved tx")
	}

	if err := Init(&s); err != nil {
		t.Error(s)
	}

	if err := CheckAddTx(&s, tr1, tx.TxNormal); err != nil {
		t.Error(err)
	}
	trs, err2 = Resolve(&s)
	if err2 != nil {
		t.Error(err2)
	}
	if len(trs) != 2 {
		t.Fatal("invalid resolved tx", len(trs))
	}

	tr3 := tx.NewMinableFee(s.Config, genesis[0])
	tr3.AddInput(tr1.Hash(), 0)
	if err := tr3.AddOutput(s.Config, a.Address58(), aklib.ADKSupply-100); err != nil {
		t.Error(err)
	}
	if err := tr3.AddOutput(s.Config, "", 100); err != nil {
		t.Error(err)
	}
	if err := tr3.Sign(a); err != nil {
		t.Error(err)
	}

	if err := CheckAddTx(&s, tr3, tx.TxRewardFee); err != nil {
		t.Error(err)
	}
	trs, err2 = Resolve(&s)
	if err2 != nil {
		t.Error(err2)
	}
	if len(trs) != 1 {
		t.Error("invalid resolved tx", len(trs))
	}

	it, err2 := tx.IssueTicket(s.Config, a, genesis[0])
	if err2 != nil {
		t.Error(err2)
	}

	tr4 := tx.NewMinableTicket(s.Config, it.Hash(), genesis[0])
	tr4.AddInput(tr1.Hash(), 0)
	if err := tr4.AddOutput(s.Config, a.Address58(), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tr4.Sign(a); err != nil {
		t.Error(err)
	}

	if err := CheckAddTx(&s, tr4, tx.TxRewardTicket); err != nil {
		t.Error(err)
	}
	trs, err2 = Resolve(&s)
	if err2 != nil {
		t.Error(err2)
	}
	if len(trs) != 0 {
		t.Error("invalid resolved tx", len(trs))
	}
}

func TestImesh2(t *testing.T) {
	setup(t)
	defer teardown(t)
	tr := tx.New(s.Config, genesis[0])
	tr.AddInput(genesis[0], 0)
	if err := tr.AddOutput(s.Config, a.Address58(), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(a); err != nil {
		t.Error(err)
	}
	if err := tr.PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, tr, tx.TxNormal); err != nil {
		t.Error(err)
	}
	trs, err2 := Resolve(&s)
	if err2 != nil {
		t.Error(err2)
	}
	if len(trs) != 1 {
		t.Error("invalid resolved txs")
	}

	tr1 := tx.NewMinableFee(s.Config, genesis[0])
	tr1.AddInput(tr.Hash(), 0)
	if err := tr1.AddOutput(s.Config, a.Address58(), aklib.ADKSupply-10); err != nil {
		t.Error(err)
	}
	if err := tr1.AddOutput(s.Config, "", 10); err != nil {
		t.Error(err)
	}
	if err := tr1.Sign(a); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, tr1, tx.TxRewardFee); err != nil {
		t.Error(err)
	}
	trs, err2 = Resolve(&s)
	if err2 != nil {
		t.Error(err2)
	}
	if len(trs) != 1 {
		t.Error("invalid resolved txs")
	}
	if _, err := GetMinableTx(&s, tr1.Hash(), tx.TxRewardFee); err != nil {
		t.Error(err)
	}
	ts, err2 := GetRandomMinableTx(&s, tx.TxRewardFee)
	if err2 != nil {
		t.Error(err2)
	}
	if !bytes.Equal(ts.Hash(), tr1.Hash()) {
		t.Error("invalid get random")
	}
	tr2 := tx.New(s.Config, genesis[0])
	tr2.AddInput(tr.Hash(), 0)
	if err := tr2.AddOutput(s.Config, a.Address58(), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tr2.Sign(a); err != nil {
		t.Error(err)
	}
	if err := tr2.PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, tr2, tx.TxNormal); err != nil {
		t.Error(err)
	}
	trs, err2 = Resolve(&s)
	if err2 != nil {
		t.Error(err2)
	}
	if len(trs) != 1 {
		t.Error("invalid resolved txs", len(trs))
	}

	if _, err := GetMinableTx(&s, tr1.Hash(), tx.TxRewardFee); err == nil {
		t.Error("should be error")
	}
	valid, err2 := IsMinableTxValid(&s, tr1)
	if err2 != nil {
		t.Error(err2)
	}
	if valid {
		t.Error("invalid validator")
	}
}

func TestImesh3(t *testing.T) {
	setup(t)
	defer teardown(t)
	var zero [32]byte
	var one [32]byte
	one[0] = 1
	tr := tx.New(s.Config, zero[:])
	if err := tr.PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, tr, tx.TxNormal); err != nil {
		t.Error(err)
	}
	trs, err2 := Resolve(&s)
	if err2 != nil {
		t.Error(err2)
	}
	if len(trs) != 0 {
		t.Error("invalid resolved txs", len(trs))
	}

	ne, err2 := GetSearchingTx(&s)
	if err2 != nil {
		t.Error(err2)
	}
	if len(ne) != 1 {
		t.Error("invalid searching tx", len(ne))
	}
	if !bytes.Equal(ne[0].Hash, zero[:]) {
		t.Error("invalid searching tx")
	}

	if err := AddNoexistTxHash(&s, one[:], tx.TxNormal); err != nil {
		t.Error(err)
	}
	ne, err2 = GetSearchingTx(&s)
	if err2 != nil {
		t.Error(err2)
	}
	if len(ne) != 1 {
		t.Error("invalid searching tx", len(ne))
	}
	if !bytes.Equal(ne[0].Hash, one[:]) {
		t.Error("invalid searching tx")
	}
	if unresolved.Noexists[one].Count != 1 {
		t.Error("invalid count")
	}
	for i := 0; i < 10; i++ {
		unresolved.Noexists[one].Searched = time.Now().Add(-24 * time.Hour)
		ne, err2 = GetSearchingTx(&s)
		if err2 != nil {
			t.Error(err2)
		}
		if len(ne) != 1 {
			t.Error("invalid searching tx", len(ne), i)
		}
	}
	if _, e := unresolved.Noexists[one]; e {
		t.Error("should be removed")
	}
	broken, err2 := isBrokenTx(&s, one[:])
	if err2 != nil {
		t.Error(err2)
	}
	if !broken {
		t.Error("should be broken")
	}
}

func TestImesh4(t *testing.T) {
	setup(t)
	defer teardown(t)

	hs, err2 := GetTxsFromAddress(&s, a.Address())
	if err2 != nil {
		t.Error(err2)
	}
	if len(hs) != 1 {
		t.Error("length should be 1")
	}
	if !bytes.Equal(hs[0], genesis[0]) {
		t.Error("should be equal")
	}
	seed := address.GenerateSeed()
	a1, err2 := address.New(address.Height10, seed, s.Config)
	if err2 != nil {
		t.Error(err2)
	}
	tr := tx.New(s.Config, genesis[0])
	tr.AddInput(genesis[0], 0)
	if err := tr.AddOutput(s.Config, a1.Address58(), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(a); err != nil {
		t.Error(err)
	}
	if err := tr.PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, tr, tx.TxNormal); err != nil {
		t.Error(err)
	}

	seed = address.GenerateSeed()
	a2, err2 := address.New(address.Height10, seed, s.Config)
	if err2 != nil {
		t.Error(err2)
	}
	tr2 := tx.New(s.Config, genesis[0])
	tr2.AddInput(tr.Hash(), 0)
	if err := tr2.AddOutput(s.Config, a2.Address58(), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tr2.Sign(a1); err != nil {
		t.Error(err)
	}
	if err := tr2.PoW(); err != nil {
		t.Error(err)
	}
	if err := CheckAddTx(&s, tr2, tx.TxNormal); err != nil {
		t.Error(err)
	}
	txs, err2 := Resolve(&s)
	if err2 != nil {
		t.Error(err2)
	}
	if len(txs) != 2 {
		t.Error("invalid length of txs")
	}

	hs, err2 = GetTxsFromAddress(&s, a.Address())
	if err2 != nil {
		t.Error(err2)
	}
	if len(hs) != 1 {
		t.Error("length should be 1")
	}
	if !bytes.Equal(hs[0], tr.Hash()) {
		t.Error("should be equal")
	}
	hs, err2 = GetTxsFromAddress(&s, a1.Address())
	if err2 != nil {
		t.Error(err2)
	}
	if len(hs) != 1 {
		t.Error("length should be 1")
	}
	if !bytes.Equal(hs[0], tr2.Hash()) {
		t.Error("should be equal")
	}
	hs, err2 = GetTxsFromAddress(&s, a2.Address())
	if err2 != nil {
		t.Error(err2)
	}
	if len(hs) != 1 {
		t.Error("length should be 1")
	}
	if !bytes.Equal(hs[0], tr2.Hash()) {
		t.Error("should be equal")
	}
}
