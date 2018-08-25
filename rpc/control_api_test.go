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
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/consensus"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/node"

	"github.com/dgraph-io/badger"
)

func TestControlAPI(t *testing.T) {
	setup(t)
	defer teardown(t)

	pwd := []byte("pwd")
	if err := InitSecret(&s, pwd); err != nil {
		t.Error(err)
	}
	if err := decryptSecret(&s, pwd); err != nil {
		t.Error(err)
	}
	GoNotify(&s, node.RegisterTxNotifier, consensus.RegisterTxNotifier)
	acs := []string{"ac1"}
	var adr string
	for _, ac := range acs {
		for _, adr = range newAddressT(t, ac) {
			t.Log(adr)
		}
	}
	tr := tx.New(s.Config, genesis)
	tr.AddInput(genesis, 0)
	if err := tr.AddOutput(s.Config, a.Address58(s.Config), aklib.ADKSupply-10); err != nil {
		t.Error(err)
	}
	if err := tr.AddOutput(s.Config, adr, 10); err != nil {
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

	testdumpseed(t)
	teststop(t)
	testdumpwallet(t)
	testimportwallet(t, pwd)

	to := net.JoinHostPort(s.Bind, strconv.Itoa(int(s.Port)))
	conn, err2 := net.DialTimeout("tcp", to, 3*time.Second)
	if err2 != nil {
		t.Error(err2)
	}
	tcpconn, ok := conn.(*net.TCPConn)
	if !ok {
		t.Error("invalid connection")
	}
	if err := tcpconn.SetDeadline(time.Now().Add(time.Minute)); err != nil {
		t.Error(err)
	}

	v := msg.NewVersion(&s1, *msg.NewAddr(to, msg.ServiceFull), 0)
	if err := msg.Write(&s1, v, msg.CmdVersion, conn); err != nil {
		t.Error(err)
	}

	cmd, _, err2 := msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdVerack {
		t.Error("message must be verack after Version")
	}

	cmd, buf, err2 := msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdVersion {
		t.Error("cmd must be version for handshake")
	}
	_, err2 = msg.ReadVersion(&s1, buf, 0)
	if err2 != nil {
		t.Error(err2)
	}
	if err := msg.Write(&s1, nil, msg.CmdVerack, conn); err != nil {
		t.Error(err)
	}
	time.Sleep(5 * time.Second)
	testlistpeer(t, 1)
	ni := testgetnodeinfo(t)
	if ni.Connections != 1 {
		t.Error("invalid nodeinfo")
	}

	bad := tx.New(s.Config, genesis)
	txd := msg.Txs{
		&msg.Tx{
			Type: msg.InvTxNormal,
			Tx:   bad,
		},
	}
	if err := msg.Write(&s1, &txd, msg.CmdTxs, conn); err != nil {
		t.Error(err)
	}
	time.Sleep(3 * time.Second)
	ni = testgetnodeinfo(t)
	if ni.Connections != 0 {
		t.Error("invalid nodeinfo")
	}
	testlistpeer(t, 0)
	testlistbanned(t)
}

func testlistbanned(t *testing.T) {
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "listbanned",
		Params:  json.RawMessage{},
	}
	var resp Response
	if err := listbanned(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
	bs, ok := resp.Result.([]*Bans)
	if !ok {
		t.Error("invalid return")
	}
	if len(bs) != 1 {
		t.Error("invalid listbanned")
	}
	if !strings.HasPrefix(bs[0].Address, "127.0.0.1") &&
		!strings.HasPrefix(bs[0].Address, "[::1]") {
		t.Error("invalid banned", bs[0])
	}
	if bs[0].Until-bs[0].Created != 60*60 {
		t.Error("invalid banned")
	}
	if bs[0].Created < time.Now().Add(-time.Minute).Unix() ||
		bs[0].Created > time.Now().Unix() {
		t.Error("invalid banned")
	}
}
func testlistpeer(t *testing.T, n int) {
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "listpeer",
		Params:  json.RawMessage{},
	}
	var resp Response
	if err := listpeer(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
	peers, ok := resp.Result.([]msg.Addr)
	if !ok {
		t.Error("invalid return")
	}
	if len(peers) != n {
		t.Error("invalid peerlist")
	}
	if n == 0 {
		return
	}
	if peers[0].Service != 0 {
		t.Error("invalid peerlist")
	}
	if !strings.HasPrefix(peers[0].Address, "127.0.0.1:") &&
		!strings.HasPrefix(peers[0].Address, "[::1]:") {
		t.Error("invalid peerlist")
	}
}

func testdumpseed(t *testing.T) {
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "dumpseed",
		Params:  json.RawMessage{},
	}
	var resp Response
	if err := dumpprivkey(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
	seed, ok := resp.Result.(string)
	if !ok {
		t.Error("invalid return")
	}
	r, _, err := address.HDFrom58(s.Config, seed, wallet.Secret.pwd)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(r, wallet.Secret.seed) {
		t.Error("invalid dumpseed")
	}
}

func teststop(t *testing.T) {
	s.Stop = make(chan struct{}, 2)
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "stop",
		Params:  json.RawMessage{},
	}
	var resp Response
	if err := stop(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	select {
	case <-s.Stop:
	case <-time.After(5 * time.Second):
		t.Error("invalid stop")
	}
}

func testdumpwallet(t *testing.T) {
	wdat := filepath.Join(tdir, "tmp.dat")
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "dumpwallet",
	}
	var err error
	req.Params, err = json.Marshal([]interface{}{wdat})
	if err != nil {
		t.Error(err)
	}
	t.Log(wdat, string(req.Params))
	var resp Response
	if err := dumpwallet(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	if _, err := ioutil.ReadFile(wdat); err != nil {
		t.Error(err)
	}
}

func testimportwallet(t *testing.T, pwd []byte) {
	wdat := filepath.Join(tdir, "tmp.dat")
	bu := wallet
	wallet = Wallet{
		AddressChange: make(map[string]struct{}),
		AddressPublic: make(map[string]struct{}),
	}
	wallet.Secret.pwd = pwd
	hist, err := getHistory(&s)
	if err != nil {
		t.Error(err)
	}
	var adrs []string
	var dat [][]byte
	err = s.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte{byte(db.HeaderWalletAddress)}
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			adrs = append(adrs, string(it.Item().Key()[1:]))
			v, err2 := it.Item().ValueCopy(nil)
			if err2 != nil {
				return err2
			}
			dat = append(dat, v)
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	if err2 := os.RemoveAll("./test_db"); err2 != nil {
		t.Error(err2)
	}
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "importwallet",
		Params:  json.RawMessage{},
	}
	req.Params, err = json.Marshal([]interface{}{wdat})
	if err != nil {
		t.Error(err)
	}
	var resp Response
	if err2 := importwallet(&s, req, &resp); err2 != nil {
		t.Error(err2)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	if !bytes.Equal(bu.Secret.EncSeed, wallet.Secret.EncSeed) {
		t.Error("invalid encseed")
	}
	if bu.Pool.Index != wallet.Pool.Index {
		t.Error("invalid pool index")
	}
	if len(bu.Pool.Address) != len(wallet.Pool.Address) {
		t.Error("invalid pool address")
	}
	for i := range bu.Pool.Address {
		if bu.Pool.Address[i] != wallet.Pool.Address[i] {
			t.Error("invalid pool address")
		}
	}
	if len(bu.AddressChange) != len(wallet.AddressChange) {
		t.Error("invalid account address")
	}
	if len(bu.AddressPublic) != len(wallet.AddressPublic) {
		t.Error("invalid account address")
	}
	for adr := range bu.AddressChange {
		if _, ok := wallet.AddressChange[adr]; !ok {
			t.Error("invalid account address")
		}
	}
	for adr := range bu.AddressPublic {
		if _, ok := wallet.AddressPublic[adr]; !ok {
			t.Error("invalid account address")
		}
	}

	hist2, err := getHistory(&s)
	if err != nil {
		t.Error(err)
	}
	if len(hist) != len(hist2) {
		t.Error("invalid hist")
	}
	for i, h := range hist {
		h2 := hist2[i]
		if h.Serialize() != h2.Serialize() {
			t.Error("invalid hist")
		}
	}
	err = s.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte{byte(db.HeaderWalletAddress)}
		i := 0
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			adr := it.Item().Key()[1:]
			if adrs[i] != string(adr) {
				t.Error("invalid address", adrs[i], string(adr))
			}
			v, err2 := it.Item().Value()
			if err2 != nil {
				return err2
			}
			if !bytes.Equal(dat[i], v) {
				t.Error("invalid address")
			}
			i++
		}
		if i != len(adrs) {
			t.Error("invalid address")
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
}
