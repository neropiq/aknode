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

package rpc

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/AidosKuneen/aidosd/aidos"
	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/node"
	"github.com/AidosKuneen/aknode/setting"
)

var s, s1 setting.Setting
var a *address.Address
var genesis tx.Hash
var l net.Listener
var tdir string

func setup(t *testing.T) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	var err error
	tdir, err = ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	var err2 error
	if err := os.RemoveAll("./test_db"); err != nil {
		log.Println(err)
	}
	s.DB, err2 = db.Open("./test_db")
	if err2 != nil {
		panic(err2)
	}

	s.Config = aklib.DebugConfig
	s.MaxConnections = 1
	s.Bind = "127.0.0.1"
	s.Port = 9624
	s.MyHostPort = ":9624"
	seed := address.GenerateSeed()
	a, err2 = address.New(address.Height10, seed, s.Config)
	if err2 != nil {
		t.Error(err2)
	}
	s.Config.Genesis = map[string]uint64{
		a.Address58(): aklib.ADKSupply,
	}
	s.RPCUser = "user"
	s.RPCPassword = "user"
	s.RPCTxTag = "test"
	s.RPCBind = "127.0.0.1"
	s.RPCPort = s.Config.DefaultRPCPort
	s.WalletNotify = "echo %s"
	leaves.Init(&s)
	if err := imesh.Init(&s); err != nil {
		t.Error(err)
	}
	gs := leaves.Get(1)
	if len(gs) != 1 {
		t.Error("invalid genesis")
	}
	genesis = gs[0]

	s1.Config = aklib.DebugConfig
	s1.MaxConnections = 1
	s1.Port = 2345
	s1.MyHostPort = ":2345"

	l, err = node.Start(&s, true)
	if err != nil {
		t.Error(err)
	}
	wallet.Secret.EncSeed = nil
	wallet.Pool.Address = nil
	wallet.Pool.Index = 0
	wallet.Accounts = make(map[string]*account)
}

func teardown(t *testing.T) {
	if err := os.RemoveAll("./test_db"); err != nil {
		t.Error(err)
	}
	if err := os.RemoveAll(tdir); err != nil {
		t.Log(err)
	}
	if err := l.Close(); err != nil {
		t.Log(err)
	}
}

type postparam struct {
	body string
	resp interface{}
}

func (p *postparam) post(usr, pwd string) error {
	client := &http.Client{}

	auth := base64.StdEncoding.EncodeToString([]byte(usr + ":" + pwd))
	ipport := fmt.Sprintf("http://localhost:%v", s.RPCPort)
	req, err := http.NewRequest("POST", ipport, bytes.NewBuffer([]byte(p.body)))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Basic "+auth)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dat, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if s, ok := p.resp.(*string); ok {
		*s = string(dat)
		return nil
	}
	return json.Unmarshal(dat, p.resp)
}

func TestAPIFee(t *testing.T) {
	setup(t)
	defer teardown(t)
	Run(&s)
	str := ""
	setfee := &postparam{
		body: `{"jsonrpc": "1.0", "id":"curltest", "method": "settxfee", "params": [0.00001] }`,
		resp: &str,
	}
	if err := setfee.post("mou", "damepo"); err != nil {
		t.Error(err)
	}
	if str != "Unauthorized\n" {
		t.Error("should be error")
		t.Log(str)
	}

	resp := &struct {
		Result bool       `json:"result"`
		Error  *aidos.Err `json:"error"`
		ID     string     `json:"id"`
	}{}
	setfee2 := &postparam{
		body: `{"jsonrpc": "1.0", "id":"curltest", "method": "settxfee", "params": [0.00001] }`,
		resp: resp,
	}

	if err := setfee2.post(s.RPCUser, s.RPCPassword); err != nil {
		t.Error(err)
	}
	//	`{"result":true,"error":null,"id":"curltest"}`,
	if resp.Error != nil {
		t.Error("should not be error")
	}
	if !resp.Result {
		t.Error("result should be true")
	}
	if resp.ID != "curltest" {
		t.Error("id must be curltest")
	}
}

func TestSig(t *testing.T) {
	setup(t)
	defer teardown(t)

	pwd := []byte("pwd")
	if err := InitSecret(&s, pwd); err != nil {
		t.Error(err)
	}
	if err := decryptSecret(&s, pwd); err != nil {
		t.Error(err)
	}
	GoNotify(&s, nil)
	adrs := newAddress(t, "")
	tr := tx.New(s.Config, genesis)
	tr.AddInput(genesis, 0)
	if err := tr.AddOutput(s.Config, a.Address58(), aklib.ADKSupply-10*aklib.ADK); err != nil {
		t.Error(err)
	}
	if err := tr.AddOutput(s.Config, adrs[0], 10*aklib.ADK); err != nil {
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
	node.Resolve()
	time.Sleep(6 * time.Second)
	confirmAll(t, nil, true)
	txid := testsendtoaddress2(t, adrs[0], 0.2)
	confirmAll(t, nil, true)
	wallet.Secret.pwd = pwd
	aa, err := getAddress(&s, adrs[0])
	if err != nil {
		t.Error(err)
	}
	if aa.address.LeafNo() != 1 {
		t.Error("invalid leaf no", aa.address.LeafNo())
	}
	tr = tx.New(s.Config, genesis)
	tr.AddInput(txid, 0)
	if err := tr.AddOutput(s.Config, a.Address58(), (0.2-0.1)*aklib.ADK); err != nil {
		t.Error(err)
	}
	if err := tr.AddOutput(s.Config, adrs[0], 0.1*aklib.ADK); err != nil {
		t.Error(err)
	}
	if err := aa.address.SetLeafNo(5); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(aa.address); err != nil {
		t.Fatal(err)
	}
	if err := tr.PoW(); err != nil {
		t.Error(err)
	}
	if err := imesh.CheckAddTx(&s, tr, tx.TypeNormal); err != nil {
		t.Fatal(err)
	}
	node.Resolve()
	time.Sleep(6 * time.Second)
	confirmAll(t, nil, true)
	testsendtoaddress2(t, adrs[1], 9.9)
	confirmAll(t, nil, true)
	aa, err = getAddress(&s, adrs[0])
	if err != nil {
		t.Error(err)
	}
	if aa.address.LeafNo() != 7 {
		t.Error("invalid leaf no", aa.address.LeafNo())
	}
}

func testsendtoaddress2(t *testing.T, adr1 string, v float64) tx.Hash {
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "sendtoaddress",
		Params:  []interface{}{adr1, v},
	}
	var resp Response
	err := sendtoaddress(&s, req, &resp)
	if err != nil {
		t.Error(err)
	}
	txid, ok := resp.Result.(string)
	if !ok {
		t.Error("invalid resp")
	}
	h, err := hex.DecodeString(txid)
	if err != nil {
		t.Error(err)
	}
	return h
}
