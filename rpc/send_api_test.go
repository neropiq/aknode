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
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/AidosKuneen/aknode/consensus"
	"github.com/AidosKuneen/aknode/imesh/leaves"

	"github.com/AidosKuneen/aknode/msg"
	"github.com/AidosKuneen/aknode/setting"

	"github.com/AidosKuneen/aklib"
	"github.com/AidosKuneen/aklib/tx"
	"github.com/AidosKuneen/aknode/imesh"
	"github.com/AidosKuneen/aknode/node"
)

func TestSendAPI(t *testing.T) {
	setup(t)
	defer teardown(t)
	ni2 := testgetnodeinfo(t)
	t.Log(ni2.TxNo)
	pwd := []byte("pwd")
	if err := InitSecret(&s, pwd); err != nil {
		t.Error(err)
	}
	if err := decryptSecret(&s, pwd); err != nil {
		t.Error(err)
	}
	clearSecret()
	GoNotify(&s, node.RegisterTxNotifier, consensus.RegisterTxNotifier)
	acs := []string{"ac1", ""}
	adr2ac := make(map[string]string)
	adr2val := make(map[string]uint64)
	ac2val := make(map[string]uint64)
	ac2adr := make(map[string][]string)
	var total uint64
	for _, ac := range acs {
		for _, adr := range newAddressT(t, ac) {
			t.Log(adr)
			adr2ac[adr] = ac
			adr2val[adr] = 10 * aklib.ADK
			ac2val[ac] += adr2val[adr]
			ac2adr[ac] = append(ac2adr[ac], adr)
			total += adr2val[adr]
		}
	}
	outadrs := newAddressT(t, "")
	outadrs0 := newAddressT(t, "")
	tr := tx.New(s.Config, genesis)
	tr.AddInput(genesis, 0)
	if err := tr.AddOutput(s.Config, a.Address58(s.Config), aklib.ADKSupply-total); err != nil {
		t.Error(err)
	}
	for adr, v := range adr2val {
		if err := tr.AddOutput(s.Config, adr, v); err != nil {
			t.Error(err)
		}
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
	params := []interface{}{
		"ac1",
		map[string]float64{
			ac2adr[""][0]: 0.1,
		},
	}
	reqParams, err := json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	err = sendmany(&s, &Request{Params: reqParams}, nil)
	if err.Error() != "not priviledged" {
		t.Error("should be error", err)
	}
	if testwalletpassphrase1(string("aa"), 0); err == nil {
		t.Error("should be error")
	}
	testwalletpassphrase2(t, string(pwd))
	testsendmany(t, true, "", "", adr2ac)
	confirmAll(t, nil, true)
	if err := walletlock(&s, nil, nil); err != nil {
		t.Error(err)
	}
	testsendmany(t, true, "", "", adr2ac)

	testwalletpassphrase2(t, string(pwd))
	testsendmany(t, false, outadrs[0], outadrs[1], adr2ac)
	testsendfrom(t, outadrs[2], adr2ac)
	testsendtoaddress(t, outadrs0[0], 0.2)

	ni := testgetnodeinfo(t)
	if *ni.Balance != total {
		t.Error("invalid nodeinfo")
	}
	if ni.TxNo != 5 {
		t.Error("invalid txno", ni.TxNo)
	}
	time.Sleep(5 * time.Second) //wait for finishing walletnotify
}

func testgetnodeinfo(t *testing.T) *NodeInfo {
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "getrawtx",
	}
	var resp Response
	if err := getnodeinfo(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	result, ok := resp.Result.(*NodeInfo)
	if !ok {
		t.Error("result must be slice")
	}
	if result.Version != setting.Version {
		t.Error("invalid version")
	}
	if result.ProtocolVersion != msg.MessageVersion {
		t.Error("invalid protocol version")
	}
	if result.WalletVersion != walletVersion {
		t.Error("invalid wallet version")
	}
	if result.Proxy != s.Proxy {
		t.Error("invalid proxy")
	}
	if result.Testnet != s.Testnet {
		t.Error("invalid testnet")
	}
	if result.KeyPoolSize != len(wallet.Pool.Address) {
		t.Error("invalid pool size")
	}
	if result.Leaves != leaves.Size() {
		t.Error("invalid leave size")
	}
	return result
}

func getDiff(t *testing.T, u0, u1 []*tx.UTXO) map[string]int64 {
	diff := make(map[string]int64)

	bal0 := make(map[string]int64)
	for _, u := range u0 {
		bal0[u.Address.String()] += int64(u.Value)
	}
	bal1 := make(map[string]int64)
	for _, u := range u1 {
		bal1[u.Address.String()] += int64(u.Value)
	}
	for adr, val := range bal0 {
		if v := bal1[adr] - val; v != 0 {
			diff[adr] = v
			t.Log(adr, v)
		}
	}
	for adr, val := range bal1 {
		if v := val - bal0[adr]; v != 0 {
			diff[adr] = v
			t.Log(adr, v)
		}
	}
	return diff
}

func checkResponse(t *testing.T, diff map[string]int64, acc string,
	resp *Response, sendto map[string]uint64, isConf bool) tx.Hash {
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	result, ok := resp.Result.(string)
	if !ok {
		t.Error("result must be string")
	}
	txid, err := hex.DecodeString(result)
	if err != nil {
		t.Error(err)
	}
	tx, err := imesh.GetTx(s.DB, txid)
	if err != nil {
		t.Error(err, txid, result)
	}
	for i, out := range tx.Outputs {
		t.Log("out", i, out.Address, out.Value)
		v, ok := sendto[out.Address.String()]
		if !ok && i == len(tx.Outputs)-1 {
			continue
		}
		if !ok && i != len(tx.Outputs)-1 {
			t.Error("invalid output #", i)
		}
		if out.Value != v {
			t.Error("invalid value", out.Value, v)
		}
		vd, ok := diff[out.Address.String()]
		if isConf {
			if !ok {
				t.Error("invalid change address", out.Address)
			}
			if out.Value != uint64(vd) {
				t.Error("invalid value", out.Value, vd)
			}
		} else {
			if ok {
				t.Error("output should not be utxo")
			}
		}
	}
	for i, in := range tx.Inputs {
		out, err := imesh.PreviousOutput(&s, in)
		if err != nil {
			t.Error(err)
		}
		t.Log("in", i, out.Address, out.Value)

		v, ok := diff[out.Address.String()]
		if !ok {
			continue
		}
		if out.Value != uint64(-v) {
			t.Error("invalid value")
		}
		ac, ok := findAddress(out.Address.String())
		if !ok {
			t.Error("invalid account", out.Address)
		}
		if acc != "*" && acc != ac {
			t.Error("invalid account")
		}
	}
	if len(tx.Outputs)-1 != len(sendto) && len(tx.Outputs) != len(sendto) {
		t.Error("invalid number of send address")
	}
	if isConf {
		if len(tx.Outputs)+len(tx.Inputs) != len(diff) {
			t.Error("invalid number of diff")
		}
	}
	return txid
}

func testwalletpassphrase1(pwd string, t float64) error {
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "walletpassphrase",
	}
	params := []interface{}{pwd, t}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		return err
	}
	var resp Response
	return walletpassphrase(&s, req, &resp)
}

func testwalletpassphrase2(t *testing.T, pwd string) {
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "walletpassphrase",
	}
	params := []interface{}{pwd, uint(6000)}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp Response
	if err := walletpassphrase(&s, req, &resp); err != nil {
		t.Error(err)
	}
	if resp.Error != nil {
		t.Error(resp.Error)
	}
	if resp.Result != nil {
		t.Error("should be nil")
	}
}

func testsendmany(t *testing.T, isErr bool, adr1, adr2 string, adr2ac map[string]string) {
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "sendmany",
	}
	params := []interface{}{"ac1",
		map[string]float64{
			adr1: 0.2,
			adr2: 0.3,
		},
	}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp Response
	utxo0, _, err := getAllUTXOs(&s)
	if err != nil {
		t.Error(err)
	}
	err = sendmany(&s, req, &resp)
	if isErr {
		if err == nil {
			t.Error("should be error")
		}
		return
	}
	if err != nil {
		t.Error(err)
	}
	t.Log(wallet.Secret.pwd)
	t.Log(wallet.Secret.seed)

	utxo1, _, err := getAllUTXOs(&s)
	if err != nil {
		t.Error(err)
	}
	diff := getDiff(t, utxo0, utxo1)
	checkResponse(t, diff, "ac1", &resp, map[string]uint64{
		adr1: uint64(0.2 * aklib.ADK),
		adr2: uint64(0.3 * aklib.ADK),
	}, false)
	confirmAll(t, nil, true)
	utxo2, _, err := getAllUTXOs(&s)
	if err != nil {
		t.Error(err)
	}
	diff = getDiff(t, utxo0, utxo2)
	checkResponse(t, diff, "ac1", &resp, map[string]uint64{
		adr1: uint64(0.2 * aklib.ADK),
		adr2: uint64(0.3 * aklib.ADK),
	}, true)

}

func testsendtoaddress(t *testing.T, adr1 string, v float64) tx.Hash {
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "sendtoaddress",
	}
	params := []interface{}{adr1, v}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp Response
	utxo0, _, err := getAllUTXOs(&s)
	if err != nil {
		t.Error(err)
	}
	err = sendtoaddress(&s, req, &resp)
	if err != nil {
		t.Error(err)
	}
	utxo1, _, err := getAllUTXOs(&s)
	if err != nil {
		t.Error(err)
	}

	diff := getDiff(t, utxo0, utxo1)
	checkResponse(t, diff, "*", &resp, map[string]uint64{
		adr1: uint64(0.2 * aklib.ADK),
	}, false)
	confirmAll(t, nil, true)
	utxo2, _, err := getAllUTXOs(&s)
	if err != nil {
		t.Error(err)
	}
	diff = getDiff(t, utxo0, utxo2)
	return checkResponse(t, diff, "*", &resp, map[string]uint64{
		adr1: uint64(0.2 * aklib.ADK),
	}, true)
}

func testsendfrom(t *testing.T, adr1 string, adr2ac map[string]string) {
	req := &Request{
		JSONRPC: "1.0",
		ID:      "curltest",
		Method:  "sendfrom",
	}
	params := []interface{}{"ac1", adr1, 0.2}
	var err error
	req.Params, err = json.Marshal(params)
	if err != nil {
		t.Error(err)
	}
	var resp Response
	utxo0, _, err := getAllUTXOs(&s)
	if err != nil {
		t.Error(err)
	}
	err = sendfrom(&s, req, &resp)
	if err != nil {
		t.Error(err)
	}
	utxo1, _, err := getAllUTXOs(&s)
	if err != nil {
		t.Error(err)
	}
	diff := getDiff(t, utxo0, utxo1)
	checkResponse(t, diff, "ac1", &resp, map[string]uint64{
		adr1: uint64(0.2 * aklib.ADK),
	}, false)
	confirmAll(t, nil, true)
	utxo2, _, err := getAllUTXOs(&s)
	if err != nil {
		t.Error(err)
	}
	diff = getDiff(t, utxo0, utxo2)
	checkResponse(t, diff, "ac1", &resp, map[string]uint64{
		adr1: uint64(0.2 * aklib.ADK),
	}, true)
}
