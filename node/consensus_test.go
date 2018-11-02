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
	"bytes"
	"context"
	"encoding/hex"
	"log"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/AidosKuneen/consensus"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/address"
	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/akconsensus"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/msg"
)

func TestConsensus(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	setup(ctx, t)
	defer teardown(t)
	defer cancel()

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

	//peer
	seed := address.GenerateSeed32()
	akseed := address.HDSeed58(s.Config, seed, []byte(""), true)
	pub, err := address.NewNode(s.Config, seed)
	if err != nil {
		t.Error(err)
	}
	//me
	seed = address.GenerateSeed32()
	akseed1 := address.HDSeed58(s.Config, seed, []byte(""), true)
	pub1, err := address.NewNode(s.Config, seed)
	if err != nil {
		t.Error(err)
	}
	//peer
	s.RunValidator = true
	s.ValidatorSecret = akseed
	s.TrustedNodes = []string{pub1.Address58(s.Config)}

	//me
	s1.RunValidator = true
	s1.ValidatorSecret = akseed1
	s1.TrustedNodes = []string{pub.Address58(s.Config)}

	if err := akconsensus.Init(ctx, &s, &ConsensusPeer{}); err != nil {
		t.Error(err)
	}

	_, err2 := start(ctx, &s)
	if err2 != nil {
		t.Error(err2)
	}

	if err2 = startConsensus(ctx, &s); err2 != nil {
		t.Error(err2)
	}
	//ignore propose
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
	/*
			cmd, buf, err2 = msg.ReadHeader(&s1, conn)
			if err2 != nil {
				t.Error(err2)
			}
			if cmd != msg.CmdProposal {
				t.Error("cmd must be proposal", cmd)
			}
			prop, noexist, err2 := akconsensus.ReadProposal(&s1, peers.cons, buf)
			if err2 != nil {
				t.Error(err2)
			}
			if !noexist {
				t.Error("invalid proposal")
			}
			if !bytes.Equal(prop.NodeID[:], pub.Address(s1.Config)[2:]) {
				t.Error("invalid proposal node id")
			}
			if prop.ProposeSeq != 0 {
				t.Error("invalid proposal seq", prop.ProposeSeq)
			}

		if prop.Position != null.ID() {
			t.Error("invalid proposal position",
				hex.EncodeToString(prop.Position[:]), tr.Hash())
		}
	*/

	null := make(consensus.TxSet)
	var nodeid consensus.NodeID
	copy(nodeid[:], pub1.Address(s1.Config)[2:])
	pro := consensus.Proposal{
		PreviousLedger: consensus.GenesisID,
		Position:       null.ID(),
		CloseTime:      time.Now(),
		Time:           time.Now(),
		NodeID:         nodeid,
		ProposeSeq:     0,
	}
	id := pro.ID()
	adr, err := s1.ValidatorAddress()
	if err != nil {
		log.Println(err)
		return
	}
	sig, err := adr.Sign(id[:])
	if err != nil {
		log.Println(err)
		return
	}
	pro.Signature = arypack.Marshal(sig)
	if err := msg.Write(&s1, pro, msg.CmdProposal, conn); err != nil {
		t.Error(err)
	}
	//wait for ledger1
	time.Sleep(5 * time.Second)

	//proposal echo
	cmd, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdProposal {
		t.Error("cmd must be proposal", cmd)
	}
	_, noexist, err2 := akconsensus.ReadProposal(&s1, peers.cons, buf)
	if err2 != nil {
		t.Error(err2)
	}
	if noexist {
		t.Error("invalid proposal")
	}

	cmd, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	//maybe changed position and sent
	if cmd == msg.CmdProposal {
		cmd, buf, err2 = msg.ReadHeader(&s1, conn)
		if err2 != nil {
			t.Error(err2)
		}
	}

	if cmd != msg.CmdValidation {
		t.Error("cmd must be validation", cmd)
	}
	val, noexist, err2 := akconsensus.ReadValidation(&s1, peers.cons, buf)
	if err2 != nil {
		t.Error(err2)
	}
	if !noexist {
		t.Error("invalid validator")
	}
	if !bytes.Equal(val.NodeID[:], pub.Address(s1.Config)[2:]) {
		t.Error("invalid validator node id")
	}
	if !val.Full || !val.Trusted || val.Seq != 1 {
		t.Error("invalid validator  full or trusted", val.Full, val.Trusted, val.Seq)
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
	for _, i := range rinv {
		t.Log(hex.EncodeToString(i.Hash[:]), i.Type)
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

	log.Println("end of sharing a tx", tr.Hash())

	//wait for proposal
	time.Sleep(5 * time.Second)

	cmd, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdProposal {
		t.Error("cmd must be proposal", cmd)
	}
	prop, noexist, err2 := akconsensus.ReadProposal(&s1, peers.cons, buf)
	if err2 != nil {
		t.Error(err2)
	}
	if !noexist {
		t.Error("invalid proposal")
	}
	if !bytes.Equal(prop.NodeID[:], pub.Address(s1.Config)[2:]) {
		t.Error("invalid proposal node id")
	}
	if prop.ProposeSeq != 0 {
		t.Error("invalid proposal seq", prop.ProposeSeq)
	}
	if !bytes.Equal(prop.Position[:], tr.Hash()) {
		t.Error("invalid proposal position",
			hex.EncodeToString(prop.Position[:]), tr.Hash())
	}

	if err := msg.Write(&s1, &inv, msg.CmdGetData, conn); err != nil {
		t.Error(err)
	}
	cmd, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	if cmd != msg.CmdTxs {
		t.Error("cmd must be txs", cmd)
	}
	tr2, err2 := msg.ReadTxs(buf)
	if err2 != nil {
		t.Error(err2)
	}
	if len(tr2) != 1 {
		t.Error("invalid read txs", cmd)
	}
	if tr2[0].Tx.Hash().Array() != tr.Hash().Array() {
		t.Error("invalid tx")
	}
	if tr2[0].Type != msg.InvTxNormal {
		t.Error("invalid type")
	}

	pro = consensus.Proposal{
		PreviousLedger: akconsensus.LatestLedger().ID(),
		Position:       consensus.TxSetID(tr.Hash().Array()),
		CloseTime:      time.Now(),
		Time:           time.Now(),
		NodeID:         nodeid,
		ProposeSeq:     0,
	}
	id = pro.ID()
	sig, err = adr.Sign(id[:])
	if err != nil {
		t.Error(err)
	}
	pro.Signature = arypack.Marshal(sig)
	if err := msg.Write(&s1, pro, msg.CmdProposal, conn); err != nil {
		t.Error(err)
	}

	//wait for validation
	time.Sleep(5 * time.Second)

	//proposal echo
	_, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}

	cmd, buf, err2 = msg.ReadHeader(&s1, conn)
	if err2 != nil {
		t.Error(err2)
	}
	//maybe changed position and sent
	if cmd == msg.CmdProposal {
		cmd, buf, err2 = msg.ReadHeader(&s1, conn)
		if err2 != nil {
			t.Error(err2)
		}
	}
	if cmd != msg.CmdValidation {
		t.Error("cmd must be validation", cmd)
	}
	val, noexist, err2 = akconsensus.ReadValidation(&s1, peers.cons, buf)
	if err2 != nil {
		t.Error(err2)
	}
	if !noexist {
		t.Error("invalid validator")
	}
	if !bytes.Equal(val.NodeID[:], pub.Address(s1.Config)[2:]) {
		t.Error("invalid validator node id")
	}
	if !val.Full || !val.Trusted || val.Seq != 2 {
		t.Error("invalid validator  full or trusted", val.Full, val.Trusted, val.Seq)
	}
	ti, err := imesh.GetTxInfo(s.DB, tr.Hash())
	if err != nil {
		t.Error(err)
	}
	if !ti.IsAccepted() {
		t.Error("incorrect confirm")
	}
}
