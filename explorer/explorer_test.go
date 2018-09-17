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
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/AidosKuneen/aklib/rand"
	"github.com/AidosKuneen/aklib/tx"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/node"
	"github.com/AidosKuneen/aknode/setting"
)

var l net.Listener
var s setting.Setting
var genesis tx.Hash
var a *address.Address

func setup(t *testing.T) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	var err error
	if err := os.RemoveAll("./test_db"); err != nil {
		log.Println(err)
	}
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
	leaves.Init(&s)
	if err := imesh.Init(&s); err != nil {
		t.Error(err)
	}
	gs := leaves.Get(1)
	if len(gs) != 1 {
		t.Error("invalid genesis")
	}
	genesis = gs[0]
	l, err = node.Start(&s, true)
	if err != nil {
		t.Error(err)
	}
	s.RunExplorer = true
	s.ExplorerBind = "0.0.0.0"
	s.ExplorerPort = 8080
	Run(&s)
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
	setup(t)
	defer teardown(t)
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
		if err := tr.AddOutput(s.Config, a.Address58(s.Config), remain); err != nil {
			t.Error(err)
		}
		if err := tr.AddOutput(s.Config, adr, v); err != nil {
			t.Error(err)
		}
		if err := tr.Sign(a); err != nil {
			t.Error(err)
		}
		if err := tr.PoW(); err != nil {
			t.Error(err)
		}
		if err := imesh.CheckAddTx(&s, tr, tx.TypeNormal); err != nil {
			t.Fatal(err)
		}
		h = tr.Hash()
		log.Println(h)
	}

	tr = tx.New(s.Config, genesis, h)
	tr.AddInput(h, 0)
	remain -= 10
	if err := tr.AddOutput(s.Config, a.Address58(s.Config), remain); err != nil {
		t.Error(err)
	}
	if err := tr.AddOutput(s.Config, adrs, 1); err != nil {
		t.Error(err)
	}
	if err := tr.AddMultisigOut(s.Config, 2, 9,
		addrs[0].Address58(s.Config), addrs[1].Address58(s.Config), addrs[2].Address58(s.Config)); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(a); err != nil {
		t.Error(err)
	}
	if err := tr.PoW(); err != nil {
		t.Error(err)
	}
	if err := imesh.CheckAddTx(&s, tr, tx.TypeNormal); err != nil {
		t.Fatal(err)
	}
	h = tr.Hash()
	log.Println(h)

	tr = tx.New(s.Config, genesis, h)
	tr.AddInput(h, 0)
	tr.AddMultisigIn(h, 0)
	remain += 9
	remain -= 10
	if err := tr.AddOutput(s.Config, a.Address58(s.Config), remain); err != nil {
		t.Error(err)
	}
	if err := tr.AddOutput(s.Config, adrs, 1); err != nil {
		t.Error(err)
	}
	if err := tr.AddMultisigOut(s.Config, 2, 9,
		addrs[0].Address58(s.Config), addrs[1].Address58(s.Config), addrs[2].Address58(s.Config)); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(a); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(addrs[0]); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(addrs[1]); err != nil {
		t.Error(err)
	}
	if err := tr.PoW(); err != nil {
		t.Error(err)
	}
	if err := imesh.CheckAddTx(&s, tr, tx.TypeNormal); err != nil {
		t.Fatal(err)
	}
	h = tr.Hash()
	log.Println(h)

	node.Resolve()
	time.Sleep(6 * time.Second)

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
	// t.Log("strl-C to stop")
	// time.Sleep(1000 * time.Hour)
}
