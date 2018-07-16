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
	"encoding/hex"
	"testing"
	"time"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
)

func TestPublicAPI(t *testing.T) {
	setup(t)
	defer teardown(t)
	testgetrawtx(t, true)
	testgetrawtx(t, false)

	tr := tx.NewMinableFee(s.Config, genesis)
	tr.AddInput(genesis, 0)
	tr.AddOutput(s.Config, a.Address58(), aklib.ADKSupply-10)
	tr.AddOutput(s.Config, "", 10)
	if err := tr.Sign(a); err != nil {
		t.Error(err)
	}
	testsendrawtx(t, tr, tx.TypeRewardFee)
	time.Sleep(3 * time.Second)
	testgetfeetx(t, float64(100)/aklib.ADK, nil)
	testgetfeetx(t, float64(10)/aklib.ADK, tr.Hash())

	ti, err := tx.IssueTicket(s.Config, a, genesis)
	if err != nil {
		t.Error(err)
	}
	testsendrawtx(t, ti, tx.TypeNormal)

	tr = tx.NewMinableTicket(s.Config, ti.Hash(), genesis)
	tr.AddInput(genesis, 0)
	if err := tr.AddOutput(s.Config, a.Address58(), aklib.ADKSupply); err != nil {
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
}

func testgethist(t *testing.T, h tx.Hash) {
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getlasthistory",
		Params:  []interface{}{a.Address58()},
	}
	var resp Response
	if err := getlasthistory(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
	is, ok := resp.Result.([]*inoutHash)
	if !ok {
		t.Error("invalid return")
	}
	if len(is) != 2 {
		t.Error("invalid length")
	}
	for _, s := range is {
		switch s.Type {
		case imesh.TypeOut:
			if s.Hash != genesis.String() {
				t.Error("invalid hash")
			}
			if s.Index != 0 {
				t.Error("invalid index")
			}
		case imesh.TypeTicketout:
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
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getleaves",
		Params:  []interface{}{},
	}
	var resp Response
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
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "sendrawtx",
		Params:  []interface{}{dat, typ},
	}
	var resp Response
	if err := sendrawtx(&s, req, &resp); err != nil {
		t.Error(err, typ)
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
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getrawtx",
		Params:  []interface{}{hex.EncodeToString(genesis), format},
	}
	var resp Response
	if err := getrawtx(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
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
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getminabletx",
		Params:  []interface{}{min},
	}
	var resp Response
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
	t.Log(resp.Result)
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
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getminabletx",
		Params:  []interface{}{"ticket"},
	}
	var resp Response
	if err := getminabletx(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
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
