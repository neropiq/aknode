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

package node

import (
	"log"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/db"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/imesh/leaves"
	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"
)

var s, s1 setting.Setting
var a *address.Address
var genesis tx.Hash

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
	s.MaxConnections = 1
	s.Bind = "127.0.0.1"
	s.Port = 44134
	s.MyHostPort = ":44134"
	seed := address.GenerateSeed32()
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

	s1.Config = aklib.DebugConfig
	s1.MaxConnections = 1
	s1.Port = 2345
	s1.MyHostPort = ":2345"

	nodesDB.Addrs = make(adrmap)
	peers.Peers = make(map[string]*peer)
	banned.addr = make(map[string]time.Time)
	if err := initDB(&s); err != nil {
		t.Error(err)
	}

}

func teardown(t *testing.T) {
	if err := os.RemoveAll("./test_db"); err != nil {
		t.Error(err)
	}
}
func TestNode(t *testing.T) {
	setup(t)
	defer teardown(t)
	s.Config.DNS = []aklib.SRV{
		aklib.SRV{
			Service: "seeds",
			Name:    "aidoskuneen.com",
		}}
	if err := lookup(&s); err != nil {
		t.Error(err)
	}
	if len(nodesDB.Addrs) != 4 {
		t.Error("len should be 4")
	}
	nodesDB.Addrs = nil
	if err := initDB(&s); err != nil {
		t.Error(err)
	}
	if len(nodesDB.Addrs) != 4 {
		t.Error("len should be 4")
	}
}
func TestNode2(t *testing.T) {
	setup(t)
	defer teardown(t)
	tr := tx.New(s.Config, genesis)
	tr.AddInput(genesis, 0)
	if err := tr.AddOutput(s.Config, a.Address58(s.Config), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(a); err != nil {
		t.Error(err)
	}
	if err := tr.PoW(); err != nil {
		t.Error(err)
	}

	tra2 := tx.New(s.Config, tr.Hash())
	tra2.AddInput(genesis, 0)
	if err := tra2.AddOutput(s.Config, a.Address58(s.Config), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tra2.Sign(a); err != nil {
		t.Error(err)
	}
	if err := tra2.PoW(); err != nil {
		t.Error(err)
	}

	tra3 := tx.New(s.Config, tra2.Hash())
	tra3.AddInput(genesis, 0)
	if err := tra3.AddOutput(s.Config, a.Address58(s.Config), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tra3.Sign(a); err != nil {
		t.Error(err)
	}
	if err := tra3.PoW(); err != nil {
		t.Error(err)
	}

	l, err2 := start(&s)
	if err2 != nil {
		t.Error(err2)
	}
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

	var nonce msg.Nonce
	nonce[30] = 1
	if err := msg.Write(&s1, &nonce, msg.CmdPing, conn); err != nil {
		t.Error(err)
	}
	cmd, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdPong {
		t.Error("cmd must be pong")
	}
	n, err2 := msg.ReadNonce(buf)
	if err2 != nil {
		t.Error(err2)
	}
	if *n != nonce {
		t.Error("invalid ping or poing")
	}

	if err := msg.Write(&s1, nil, msg.CmdGetAddr, conn); err != nil {
		t.Error(err)
	}
	cmd, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdAddr {
		t.Error("cmd must be addr ")
	}
	adrs, err2 := msg.ReadAddrs(&s1, buf)
	if err2 != nil {
		t.Error(err2)
	}
	if *n != nonce {
		t.Error("invalid ping or poing")
	}
	if len(*adrs) != 1 {
		t.Error("invalid adrs", len(*adrs))
	}
	if (*adrs)[0].Address != "127.0.0.1"+s1.MyHostPort {
		t.Error("invlaid remote addr", (*adrs)[0].Address)
	}

	inv := msg.Inventories{
		&msg.Inventory{
			Type: msg.InvTxNormal,
			Hash: tr.Hash().Array(),
		},
	}
	if err := msg.Write(&s1, &inv, msg.CmdInv, conn); err != nil {
		t.Error(err)
	}
	cmd, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdGetData {
		t.Error("cmd must be getdata")
	}
	rinv, err2 := msg.ReadInventories(buf)
	if err2 != nil {
		t.Error(err2)
	}
	if len(rinv) != 1 {
		t.Error("invalid inv", len(rinv))
	}
	if rinv[0].Hash != inv[0].Hash {
		t.Error("incorrect inv")
	}
	if rinv[0].Type != inv[0].Type {
		t.Error("incorrect inv")
	}
	txd := msg.Txs{
		&msg.Tx{
			Type: msg.InvTxNormal,
			Tx:   tr,
		},
	}
	if err := msg.Write(&s1, &txd, msg.CmdTxs, conn); err != nil {
		t.Error(err)
	}

	cmd, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdInv {
		t.Error("cmd must be txs")
	}
	invs2, err2 := msg.ReadInventories(buf)
	if err2 != nil {
		t.Error(err2)
	}
	if len(invs2) != 1 {
		t.Error("invalid inv")
	}
	if invs2[0].Hash != tr.Hash().Array() {
		t.Error("invalid tx")
	}

	if err := msg.Write(&s1, &inv, msg.CmdGetData, conn); err != nil {
		t.Error(err)
	}
	cmd, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdTxs {
		t.Error("cmd must be txs")
	}
	tr2, err2 := msg.ReadTxs(buf)
	if err2 != nil {
		t.Error(err2)
	}
	if len(tr2) != 1 {
		t.Error("invalid read txs")
	}
	if tr2[0].Tx.Hash().Array() != tr.Hash().Array() {
		t.Error("invalid tx")
	}
	if tr2[0].Type != msg.InvTxNormal {
		t.Error("invalid type")
	}

	var lfrom msg.LeavesFrom
	if err := msg.Write(&s1, &lfrom, msg.CmdGetLeaves, conn); err != nil {
		t.Error(err)
	}
	cmd, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdLeaves {
		t.Error("cmd must be ")
	}
	rfrom, err2 := msg.ReadInventories(buf)
	if err2 != nil {
		t.Error(err2)
	}
	if len(rfrom) != 1 {
		t.Error("invalid inv length", len(rfrom))
	}
	if rfrom[0].Hash != tr.Hash().Array() {
		t.Error("invalid leaf")
	}

	WriteAll(&s, nil, msg.CmdGetLeaves)
	cmd, _, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdGetLeaves {
		t.Error("cmd must be get leaves")
	}

	inv = msg.Inventories{
		&msg.Inventory{
			Type: msg.InvTxNormal,
			Hash: tra3.Hash().Array(),
		},
	}
	if err := msg.Write(&s1, &inv, msg.CmdLeaves, conn); err != nil {
		t.Error(err)
	}
	tras := []*tx.Transaction{tra3, tra2}
	for i := 0; i < 2; i++ {
		cmd, buf, err2 = msg.ReadHeader(&s1, conn)
		if err2 != nil {
			t.Error(err2)
		}
		if cmd != msg.CmdGetData {
			t.Error("cmd must be get data")
		}
		rinv, err2 = msg.ReadInventories(buf)
		if err2 != nil {
			t.Error(err2)
		}
		if len(rinv) != 1 {
			t.Error("invalid get data", len(rinv))
		}
		if rinv[0].Hash != tras[i].Hash().Array() {
			t.Error("invalid inv")
		}
		txd = msg.Txs{
			&msg.Tx{
				Type: msg.InvTxNormal,
				Tx:   tras[i],
			},
		}
		if err := msg.Write(&s1, &txd, msg.CmdTxs, conn); err != nil {
			t.Error(err)
		}
	}

	WriteAll(&s, nil, msg.CmdGetAddr)
	cmd, _, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdGetAddr {
		t.Error("cmd must be get addr")
	}

	addrs := msg.Addrs{
		msg.Addr{
			Service: msg.ServiceFull,
			Address: "google.com:333",
		},
	}
	if err := msg.Write(&s1, &addrs, msg.CmdAddr, conn); err != nil {
		t.Error(err)
	}
	time.Sleep(3 * time.Second)
	if len(nodesDB.Addrs) != 2 {
		t.Error("invalid adr cmd", len(nodesDB.Addrs))
	}
	if _, e := nodesDB.Addrs[addrs[0].Address]; !e {
		t.Error("didnt add adr")
	}

	bad := tx.New(s.Config, genesis)
	txd = msg.Txs{
		&msg.Tx{
			Type: msg.InvTxNormal,
			Tx:   bad,
		},
	}
	if err := msg.Write(&s1, &txd, msg.CmdTxs, conn); err != nil {
		t.Error(err)
	}
	time.Sleep(3 * time.Second)
	if _, e := banned.addr["127.0.0.1"]; !e {
		t.Error("should be banned")
	}
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Error(err)
	}
	if err := msg.Write(&s1, &nonce, msg.CmdPing, conn); err == nil {
		t.Error("should be banned")
	}
	conn, err2 = net.DialTimeout("tcp", to, 3*time.Second)
	if err2 != nil {
		t.Error(err2)
	}
	v = msg.NewVersion(&s1, *msg.NewAddr(to, msg.ServiceFull), 0)
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Error(err)
	}
	if err := msg.Write(&s1, v, msg.CmdVersion, conn); err != nil {
		t.Error(err)
	}
	if _, _, err := msg.ReadHeader(&s1, conn); err == nil {
		t.Error("should be banned")
	}
	if err := l.Close(); err != nil {
		t.Error(err)
	}
}

func TestNode3(t *testing.T) {
	setup(t)
	defer teardown(t)

	tcpAddr, err2 := net.ResolveTCPAddr("tcp", s1.MyHostPort)
	if err2 != nil {
		t.Error(err2)
	}
	l, err2 := net.ListenTCP("tcp", tcpAddr)
	if err2 != nil {
		t.Error(err2)
	}
	ch := make(chan struct{})
	go func() {
		defer func() {
			l.Close()
			ch <- struct{}{}
		}()
		conn, err3 := l.AcceptTCP()
		if err3 != nil {
			t.Error(err3)
		}
		if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Error(err)
		}
		p, err3 := readVersion(&s1, conn, 0)
		if err3 != nil {
			t.Error(err3)
		}
		if err := writeVersion(&s1, p.remote, conn, 0); err != nil {
			t.Error(err)
		}
	}()
	if err := putAddrs(&s, *msg.NewAddr("127.0.0.1"+s1.MyHostPort, msg.ServiceFull)); err != nil {
		t.Error(err)
	}
	connect(&s)
	<-ch
}
func TestNode4(t *testing.T) {
	setup(t)
	defer teardown(t)
	setReadDeadline = func(p *peer, t time.Time) error {
		return p.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	}

	l, err2 := start(&s)
	if err2 != nil {
		t.Error(err2)
	}
	time.Sleep(3 * time.Second)
	to := net.JoinHostPort(s.Bind, strconv.Itoa(int(s.Port)))
	conn, err2 := net.DialTimeout("tcp", to, 3*time.Second)
	if err2 != nil {
		t.Error(err2)
	}
	tcpconn, ok := conn.(*net.TCPConn)
	if !ok {
		t.Error("invalid connection")
	}
	if err := tcpconn.SetDeadline(time.Now().Add(20 * time.Second)); err != nil {
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
	time.Sleep(4 * time.Second)
	cmd, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdPing {
		t.Error("cmd must be ping")
	}
	n, err2 := msg.ReadNonce(buf)
	if err2 != nil {
		t.Error(err2)
	}
	if err := msg.Write(&s1, n, msg.CmdPong, conn); err != nil {
		t.Error(err)
	}
	if err := l.Close(); err != nil {
		t.Error(err)
	}
}
