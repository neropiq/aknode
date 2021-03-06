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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"

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
var a, b *address.Address
var genesis tx.Hash
var l net.Listener
var tdir string

func setup(ctx context.Context, t *testing.T) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	{
		var err error
		tdir, err = ioutil.TempDir("", "gotest")
		if err != nil {
			t.Fatal(err)
		}
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
	s.Port = uint16(rand.Int31n(10000)) + 1025
	s.MyHostPort = ":" + strconv.Itoa(int(s.Port))
	seed := address.GenerateSeed32()
	a, err2 = address.New(s.Config, seed)
	if err2 != nil {
		t.Error(err2)
	}
	seed = address.GenerateSeed32()
	b, err2 = address.New(s.Config, seed)
	if err2 != nil {
		t.Error(err2)
	}
	s.Config.Genesis = map[string]uint64{
		a.Address58(s.Config): aklib.ADKSupply,
	}
	s.RPCUser = "user"
	s.RPCPassword = "user"
	s.RPCTxTag = "test"
	s.RPCBind = "127.0.0.1"
	s.RPCPort = s.Config.DefaultRPCPort
	s.WalletNotify = "echo %s"
	if err := leaves.Init(&s); err != nil {
		t.Error(err)
	}
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
	s1.Port = uint16(rand.Int31n(10000)) + 1025
	s1.MyHostPort = ":" + strconv.Itoa(int(s1.Port))
	var err error
	l, err = node.Start(ctx, &s, true)
	if err != nil {
		t.Error(err)
	}
	t.Log(imesh.GetTxNo())
	if err := Init(&s); err != nil {
		t.Error(err)
	}
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
	ctx, cancel := context.WithCancel(context.Background())
	setup(ctx, t)
	defer teardown(t)
	defer cancel()
	Run(ctx, &s)
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
