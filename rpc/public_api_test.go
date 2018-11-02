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
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/rpc"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/consensus"
)

func TestPublicAPI(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	setup(ctx, t)
	defer teardown(t)
	defer cancel()
	testgetrawtx(t, true)
	testgetrawtx(t, false)

	tr := tx.NewMinableFee(s.Config, genesis)
	tr.AddInput(genesis, 0)
	if err := tr.AddOutput(s.Config, a.Address58(s.Config), aklib.ADKSupply-10); err != nil {
		t.Error(err)
	}
	if err := tr.AddOutput(s.Config, "", 10); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(a); err != nil {
		t.Error(err)
	}
	testsendrawtx(t, tr, tx.TypeRewardFee)
	time.Sleep(3 * time.Second)
	testgetfeetx(t, float64(100)/aklib.ADK, nil)
	testgetfeetx(t, float64(10)/aklib.ADK, tr.Hash())

	ti, err := tx.IssueTicket(context.Background(), s.Config, a.Address(s.Config), genesis)
	if err != nil {
		t.Error(err)
	}
	testsendrawtx(t, ti, tx.TypeNormal)

	tr = tx.NewMinableTicket(s.Config, ti.Hash(), genesis)
	tr.AddInput(genesis, 0)
	if err := tr.AddOutput(s.Config, a.Address58(s.Config), aklib.ADKSupply); err != nil {
		t.Error(err)
	}
	if err := tr.Sign(a); err != nil {
		t.Error(err)
	}
	testsendrawtx(t, tr, tx.TypeRewardTicket)
	time.Sleep(6 * time.Second)
	testgettickettx(t, tr.Hash())
	testgetleaves(t, ti.Hash())
	testgethist(t, ti.Hash())
	testgettxsstatus(t, ti.Hash(), false)
	confirmAll(t, nil, true)
	testgettxsstatus(t, ti.Hash(), true)
}
func TestPublicAPI3(t *testing.T) {
	testgetledger(t)
}

func TestPublicAPI2(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	setup(ctx, t)
	defer teardown(t)
	defer cancel()
	defer teardown(t)
	tr2 := tx.New(s.Config, genesis)
	tr2.AddInput(genesis, 0)
	if err := tr2.AddMultisigOut(s.Config, 1, aklib.ADKSupply, a.Address58(s.Config), b.Address58(s.Config)); err != nil {
		t.Error(err)
	}
	if err := tr2.Sign(a); err != nil {
		t.Error(err)
	}
	if err := tr2.PoW(); err != nil {
		t.Error(err)
	}
	testsendrawtx(t, tr2, tx.TypeNormal)
	time.Sleep(6 * time.Second)
	testgetmultisiginfo(t, tr2)
}

func testgetmultisiginfo(t *testing.T, tr *tx.Transaction) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getmultisiginfo",
		Params:  json.RawMessage{},
	}
	mout := tr.MultiSigOuts[0]
	params := []interface{}{mout.Address(s.Config)}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp rpc.Response
	if err := getmultisiginfo(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
	msig, ok := resp.Result.(*tx.MultisigStruct)
	if !ok {
		t.Error("invalid return")
	}
	if msig.M != 1 {
		t.Error("invalid msig")
	}
	if len(msig.Addresses) != 2 {
		t.Error("invalid msig")
	}
	if bytes.Equal(msig.Addresses[0], a.Address(s.Config)) && bytes.Equal(msig.Addresses[1], b.Address(s.Config)) ||
		bytes.Equal(msig.Addresses[1], a.Address(s.Config)) && bytes.Equal(msig.Addresses[0], b.Address(s.Config)) {
	} else {
		t.Error("invalid msig")
	}
}

func testgettxsstatus(t *testing.T, h tx.Hash, isConf bool) {
	var zero [32]byte
	var inva tx.Hash = zero[:]
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "gettxsstatus",
		Params:  json.RawMessage{},
	}
	params := []interface{}{genesis.String(), h.String(), inva.String()}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp rpc.Response
	if err := gettxsstatus(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
	is, ok := resp.Result.([]*rpc.TxStatus)
	if !ok {
		t.Error("invalid return")
	}
	if len(is) != 3 {
		t.Error("invalid return")
	}
	if !is[0].Exists || is[0].Hash != genesis.String() || is[0].IsRejected || !is[0].IsConfirmed || is[0].LedgerID != hex.EncodeToString(imesh.StatusGenesis[:]) {
		t.Error("invalid genesis status", is[0])
	}
	if isConf {
		id := ledger.ID()
		if !is[1].Exists || is[1].Hash != h.String() || is[1].IsRejected || !is[1].IsConfirmed || is[1].LedgerID != hex.EncodeToString(id[:]) {
			t.Error("invalid tx status", is[1])
		}
	}
	if !isConf {
		if !is[1].Exists || is[1].Hash != h.String() || is[1].IsRejected || is[1].IsConfirmed || is[1].LedgerID != "" {
			t.Error("invalid tx status", is[1])
		}
	}
	if is[2].Exists || is[2].Hash != inva.String() || is[2].IsRejected || is[2].IsConfirmed || is[2].LedgerID != "" {
		t.Error("invalid tx status", is[2])
	}
}

func testgethist(t *testing.T, h tx.Hash) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getlasthistory",
	}
	params := []interface{}{a.Address58(s.Config)}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp rpc.Response
	if err := getlasthistory(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
	is, ok := resp.Result.([]*rpc.InoutHash)
	if !ok {
		t.Error("invalid return")
	}
	if len(is) != 2 {
		t.Error("invalid length")
	}
	for _, s := range is {
		switch s.Type {
		case tx.TypeOut:
			if s.Hash != genesis.String() {
				t.Error("invalid hash")
			}
			if s.Index != 0 {
				t.Error("invalid index")
			}
		case tx.TypeTicketout:
			if s.Hash != h.String() {
				t.Error("invalid hash")
			}
			if s.Index != 0 {
				t.Error("invalid index")
			}
		default:
			t.Error("invalid type")
		}
	}
}

func testgetleaves(t *testing.T, l tx.Hash) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getleaves",
	}
	var resp rpc.Response
	if err := getleaves(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
	is, ok := resp.Result.([]string)
	if !ok {
		t.Error("invalid return")
	}
	if len(is) != 1 {
		t.Error("invalid length")
	}
	h, err := hex.DecodeString(is[0])
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(h, l) {
		t.Error("invalid getsendtx")
	}
}

func testsendrawtx(t *testing.T, tr *tx.Transaction, typ tx.Type) {
	dat := arypack.Marshal(tr)
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "sendrawtx",
	}
	params := []interface{}{dat, typ}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}

	var resp rpc.Response
	if err2 := sendrawtx(&s, req, &resp); err2 != nil {
		t.Error(err2, typ)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
	hs, ok := resp.Result.(string)
	if !ok {
		t.Error("invalid return")
	}
	h, err := hex.DecodeString(hs)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(tr.Hash(), h) {
		t.Error("invalid getsendtx")
	}
}

func testgetrawtx(t *testing.T, format bool) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getrawtx",
	}
	params := []interface{}{hex.EncodeToString(genesis), format}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp rpc.Response
	if err := getrawtx(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	var tr tx.Transaction
	if !format {
		dat, ok := resp.Result.([]byte)
		if !ok {
			t.Error("invalid data format")
		}
		if err := arypack.Unmarshal(dat, &tr); err != nil {
			t.Error(err)
		}
	} else {
		trr, ok := resp.Result.(*tx.Transaction)
		if !ok {
			t.Error("invalid data format")
		}
		tr = *trr
	}
	if !bytes.Equal(tr.Hash(), genesis) {
		t.Error("invalid getrawtx")
	}
}

func testgetfeetx(t *testing.T, min float64, h tx.Hash) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getminabletx",
	}
	params := []interface{}{min}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp rpc.Response
	if err := getminabletx(&s, req, &resp); err != nil {
		if h == nil {
			if err == nil {
				t.Error("should be error")
			}
			return
		}
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	var tr tx.Transaction
	dat, ok := resp.Result.([]byte)
	if !ok {
		t.Error("invalid data format")
	}
	if err := arypack.Unmarshal(dat, &tr); err != nil {
		t.Error(err)
	}
	if !bytes.Equal(tr.Hash(), h) {
		t.Error("invalid getfeetx")
	}
}

func testgettickettx(t *testing.T, h tx.Hash) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getminabletx",
	}
	params := []interface{}{"ticket"}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp rpc.Response
	if err := getminabletx(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	var tr tx.Transaction
	dat, ok := resp.Result.([]byte)
	if !ok {
		t.Error("invalid data format")
	}
	if err := arypack.Unmarshal(dat, &tr); err != nil {
		t.Error(err)
	}
	if !bytes.Equal(tr.Hash(), h) {
		t.Error("invalid gettickettx")
	}
}

func testgetledger(t *testing.T) {
	req := &rpc.Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getledger",
		Params:  json.RawMessage{},
	}
	params := []interface{}{hex.EncodeToString(consensus.GenesisID[:])}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp rpc.Response
	if err := getledger(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
	l, ok := resp.Result.(*rpc.Ledger)
	if !ok {
		t.Error("invalid return")
	}
	if l.ID != hex.EncodeToString(consensus.GenesisID[:]) {
		t.Error("invalid ledger")
	}
	if l.Seq != 0 {
		t.Error("invalid ledger", l.Seq, l.Txs)
	}
}
