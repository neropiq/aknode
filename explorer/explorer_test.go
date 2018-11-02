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

package explorer

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/AidosKuneen/aklib/rand"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/consensus"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aknode/akconsensus"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/node"
	"github.com/AidosKuneen/aknode/setting"
)

var l net.Listener
var s setting.Setting
var genesis tx.Hash
var a *address.Address

func setup(t *testing.T) context.CancelFunc {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	if err := os.RemoveAll("./test_db"); err != nil {
		log.Println(err)
	}
	var err error
	s.DB, err = db.Open("./test_db")
	if err != nil {
		panic(err)
	}

	s.Config = aklib.DebugConfig
	s.MaxConnections = 1
	s.Bind = "127.0.0.1"
	s.Port = 9624
	s.MyHostPort = ":9624"
	seed := address.GenerateSeed32()
	var err2 error
	a, err2 = address.New(s.Config, seed)
	if err2 != nil {
		t.Error(err2)
	}
	s.Config.Genesis = map[string]uint64{
		a.Address58(s.Config): aklib.ADKSupply,
	}
	if err := leaves.Init(&s); err != nil {
		t.Error(err)
	}
	if err = imesh.Init(&s); err != nil {
		t.Error(err)
	}
	gs := leaves.Get(1)
	if len(gs) != 1 {
		t.Error("invalid genesis")
	}
	genesis = gs[0]
	ctx, cancel := context.WithCancel(context.Background())

	l, err = node.Start(ctx, &s, true)
	if err != nil {
		t.Error(err)
	}
	s.RunExplorer = true
	s.ExplorerBind = "0.0.0.0"
	s.ExplorerPort = 8080
	Run(ctx, &s)
	return cancel
}

func teardown(t *testing.T) {
	if err := os.RemoveAll("./test_db"); err != nil {
		t.Error(err)
	}
	if err := l.Close(); err != nil {
		t.Log(err)
	}
}
func TestExploere(t *testing.T) {
	cancel := setup(t)
	defer teardown(t)
	defer cancel()
	log.Println(genesis)
	time.Sleep(3 * time.Second)
	cl := http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := cl.Get(fmt.Sprintf("http://localhost:%d", s.ExplorerPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("should be OK")
	}
	resp, err = cl.Get(fmt.Sprintf("http://localhost:%d/address?id=AAA", s.ExplorerPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatal("should be not found")
	}
	resp, err = cl.Get(fmt.Sprintf("http://localhost:%d/tx?id=AAA", s.ExplorerPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatal("should be not found")
	}
	resp, err = cl.Get(fmt.Sprintf("http://localhost:%d/statement?id=AAA", s.ExplorerPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatal("should be not found")
	}

	adr2val := make(map[string]uint64)
	addrs := make([]*address.Address, 3)
	for i := 0; i < 3; i++ {
		seed := address.GenerateSeed32()
		var err2 error
		addrs[i], err2 = address.New(s.Config, seed)
		if err2 != nil {
			t.Error(err2)
		}
		adr2val[addrs[i].Address58(s.Config)] = uint64(rand.R.Int31())
	}
	h := genesis
	var remain = aklib.ADKSupply
	var tr *tx.Transaction
	var adrs string
	for adr, v := range adr2val {
		t.Log(adr, v)
		adrs = adr
		tr = tx.New(s.Config, genesis)
		tr.AddInput(h, 0)
		if v >= aklib.ADKSupply {
			t.Fatal(v)
		}
		remain -= v
		if err2 := tr.AddOutput(s.Config, a.Address58(s.Config), remain); err2 != nil {
			t.Error(err2)
		}
		if err2 := tr.AddOutput(s.Config, adr, v); err2 != nil {
			t.Error(err2)
		}
		if err2 := tr.Sign(a); err2 != nil {
			t.Error(err2)
		}
		if err2 := tr.PoW(); err2 != nil {
			t.Error(err2)
		}
		if err2 := imesh.CheckAddTx(&s, tr, tx.TypeNormal); err2 != nil {
			t.Fatal(err2)
		}
		h = tr.Hash()
		log.Println(h)
	}

	tr = tx.New(s.Config, genesis, h)
	tr.AddInput(h, 0)
	remain -= 10
	if err2 := tr.AddOutput(s.Config, a.Address58(s.Config), remain); err2 != nil {
		t.Error(err2)
	}
	if err2 := tr.AddOutput(s.Config, adrs, 1); err2 != nil {
		t.Error(err2)
	}
	if err2 := tr.AddMultisigOut(s.Config, 2, 9,
		addrs[0].Address58(s.Config), addrs[1].Address58(s.Config), addrs[2].Address58(s.Config)); err2 != nil {
		t.Error(err2)
	}
	if err2 := tr.Sign(a); err2 != nil {
		t.Error(err2)
	}
	if err2 := tr.PoW(); err2 != nil {
		t.Error(err2)
	}
	if err2 := imesh.CheckAddTx(&s, tr, tx.TypeNormal); err2 != nil {
		t.Fatal(err2)
	}
	h = tr.Hash()
	log.Println(h)

	tr = tx.New(s.Config, genesis, h)
	tr.AddInput(h, 0)
	tr.AddMultisigIn(h, 0)
	remain += 9
	remain -= 10
	if err2 := tr.AddOutput(s.Config, a.Address58(s.Config), remain); err2 != nil {
		t.Error(err2)
	}
	if err2 := tr.AddOutput(s.Config, adrs, 1); err2 != nil {
		t.Error(err2)
	}
	if err2 := tr.AddMultisigOut(s.Config, 2, 9,
		addrs[0].Address58(s.Config), addrs[1].Address58(s.Config), addrs[2].Address58(s.Config)); err2 != nil {
		t.Error(err2)
	}
	if err2 := tr.Sign(a); err2 != nil {
		t.Error(err2)
	}
	if err2 := tr.Sign(addrs[0]); err2 != nil {
		t.Error(err2)
	}
	if err2 := tr.Sign(addrs[1]); err2 != nil {
		t.Error(err2)
	}
	if err2 := tr.PoW(); err2 != nil {
		t.Error(err2)
	}
	if err2 := imesh.CheckAddTx(&s, tr, tx.TypeNormal); err2 != nil {
		t.Fatal(err2)
	}
	h = tr.Hash()
	log.Println(h)

	node.Resolve()
	time.Sleep(6 * time.Second)

	ledger := &consensus.Ledger{
		ParentID:  consensus.GenesisID,
		Seq:       1,
		CloseTime: time.Now(),
	}
	id := ledger.ID()
	t.Log("ledger id", hex.EncodeToString(id[:]))
	if err = akconsensus.PutLedger(&s, ledger); err != nil {
		t.Fatal(err)
	}
	akconsensus.SetLatest(ledger)

	resp, err = cl.Get(fmt.Sprintf("http://localhost:%d", s.ExplorerPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("should be OK")
	}
	resp, err = cl.Get(fmt.Sprintf("http://localhost:%d/address?id=%s", s.ExplorerPort, adrs))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Error("should be OK")
	}
	madr := tr.Body.MultiSigOuts[0].Address(s.Config)
	t.Log(madr)
	resp, err = cl.Get(fmt.Sprintf("http://localhost:%d/maddress?id=%s", s.ExplorerPort, madr))
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Error("should be OK")
	}
	resp, err = cl.Get(fmt.Sprintf("http://localhost:%d/tx?id=%s", s.ExplorerPort, tr.Hash()))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("should be OK", tr.Hash())
	}
	resp, err = cl.Get(fmt.Sprintf("http://localhost:%d/statement?id=%s", s.ExplorerPort, hex.EncodeToString(id[:])))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("should be OK", tr.Hash())
	}
	// time.Sleep(1000 * time.Hour)
}
