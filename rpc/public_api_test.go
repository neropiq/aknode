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
	"testing"
	"time"

	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/node"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/arypack"
	"github.com/AidosKuneen/aklib/tx"
)

func TestPublicAPI1(t *testing.T) {
	setup(t)
	defer teardown(t)
	// testgetnodeinfo(t)
	testgetrawtx(t, true)
	testgetrawtx(t, false)

	tr := tx.NewMinableFee(s.Config, genesis)
	tr.AddInput(genesis, 0)
	tr.AddOutput(s.Config, a.Address58(), aklib.ADKSupply-10)
	tr.AddOutput(s.Config, "", 10)
	if err := tr.Sign(a); err != nil {
		t.Error(err)
	}
	if err := imesh.CheckAddTx(&s, tr, tx.TxRewardFee); err != nil {
		t.Error(err)
	}
	node.Resolve()
	time.Sleep(3 * time.Second)
	testgetfeetx(t, float64(100)/aklib.ADK, nil)
	testgetfeetx(t, float64(10)/aklib.ADK, tr.Hash())
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
		dat, ok := resp.Result.(string)
		if !ok {
			t.Error("invalid data format")
		}
		dec, err := base64.StdEncoding.DecodeString(dat)
		if err != nil {
			t.Error(err)
		}
		if err := arypack.Unmarshal(dec, &tr); err != nil {
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
	dat, ok := resp.Result.(string)
	if !ok {
		t.Error("invalid data format")
	}
	dec, err := base64.StdEncoding.DecodeString(dat)
	if err != nil {
		t.Error(err)
	}
	if err := arypack.Unmarshal(dec, &tr); err != nil {
		t.Error(err)
	}
	if !bytes.Equal(tr.Hash(), h) {
		t.Error("invalid getfeetx")
	}
}

/*
func testgetnodeinfo(t *testing.T) {
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getrawtx",
		Params:  []interface{}{},
	}
	var resp Response
	if err := getnodeinfo(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	t.Log(resp.Result)
	result, ok := resp.Result.(*nodeInfo)
	if !ok {
		t.Error("result must be slice")
	}
}
*/
