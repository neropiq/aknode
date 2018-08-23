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
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/msg"
)

func TestMiner(t *testing.T) {
	setup(t)
	defer teardown(t)

	s.RunFeeMiner = true
	s.RunTicketMiner = true

	ti, err := tx.IssueTicket(s.Config, genesis)
	if err != nil {
		t.Error(err)
	}

	l, err2 := start(&s)
	if err2 != nil {
		t.Error(err2)
	}
	RunMiner(&s)

	to := net.JoinHostPort(s.Bind, strconv.Itoa(int(s.Port)))
	conn, err2 := net.DialTimeout("tcp", to, 3*time.Second)
	if err2 != nil {
		t.Error(err2)
	}
	tcpconn, ok := conn.(*net.TCPConn)
	if !ok {
		t.Error("invalid connection")
	}
	if err := tcpconn.SetDeadline(time.Now().Add(10 * time.Minute)); err != nil {
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

	seed := address.GenerateSeed32()
	a3, err2 := address.NewFromSeed(s.Config, seed, false)
	if err2 != nil {
		t.Error(err2)
	}
	s.MinerAddress = a3.Address58(s.Config)
	txd := msg.Txs{
		&msg.Tx{
			Type: msg.InvTxNormal,
			Tx:   ti,
		},
	}
	if err := msg.Write(&s1, &txd, msg.CmdTxs, conn); err != nil {
		t.Error(err)
	}
	tr := tx.NewMinableTicket(s.Config, ti.Hash(), genesis)
	tr.AddInput(genesis, 0)
	if err := tr.AddOutput(s.Config, a.Address58(s.Config), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(a); err != nil {
		t.Error(err)
	}
	txd = msg.Txs{
		&msg.Tx{
			Type: msg.InvTxRewardTicket,
			Tx:   tr,
		},
	}
	if err := msg.Write(&s1, &txd, msg.CmdTxs, conn); err != nil {
		t.Error(err)
	}

	var inout []*tx.InoutHash
	for i := 0; i < 30 && len(inout) == 0; i++ {
		var err error
		time.Sleep(10 * time.Second)
		inout, err = imesh.GetHisoty(&s, a3.Address58(s.Config), true)
		if err != nil {
			t.Error(err)
		}
	}
	if len(inout) == 0 {
		t.Error("failed to mine")
	}

	seed = address.GenerateSeed32()
	a2, err2 := address.NewFromSeed(s.Config, seed, false)
	if err2 != nil {
		t.Error(err2)
	}
	s.MinerAddress = a2.Address58(s.Config)
	tr = tx.NewMinableFee(s.Config, genesis)
	tr.AddInput(inout[0].Hash, 0)
	if err := tr.AddOutput(s.Config, a.Address58(s.Config), aklib.ADKSupply-10); err != nil {
		t.Error(err)
	}
	if err := tr.AddOutput(s.Config, "", 10); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(a); err != nil {
		t.Error(err)
	}
	txd = msg.Txs{
		&msg.Tx{
			Type: msg.InvTxRewardFee,
			Tx:   tr,
		},
	}
	if err := msg.Write(&s1, &txd, msg.CmdTxs, conn); err != nil {
		t.Error(err)
	}

	inout = nil
	for i := 0; i < 30 && len(inout) == 0; i++ {
		var err error
		time.Sleep(10 * time.Second)
		inout, err = imesh.GetHisoty(&s, a2.Address58(s.Config), true)
		if err != nil {
			t.Error(err)
		}
		t.Log(len(inout))
	}
	if len(inout) == 0 {
		t.Error("failed to mine")
	}

	if err := l.Close(); err != nil {
		t.Error(err)
	}
}
